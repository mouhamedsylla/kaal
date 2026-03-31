package ssh

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

// Config holds the parameters for an SSH connection.
type Config struct {
	Host    string
	User    string
	KeyPath string
	Port    int
}

// Client wraps an SSH connection to a remote host.
type Client struct {
	conn *ssh.Client
	cfg  Config
}

// NewClient establishes an SSH connection using the given config.
//
// Key resolution order:
//  1. PILOT_SSH_KEY env var — PEM content directly (CI/CD pipelines, GitHub Actions secrets)
//  2. cfg.KeyPath file — local path, supports ~/  expansion
func NewClient(cfg Config) (*Client, error) {
	if cfg.Port == 0 {
		cfg.Port = 22
	}

	key, err := resolveSSHKey(cfg.KeyPath)
	if err != nil {
		return nil, err
	}

	signer, err := ssh.ParsePrivateKey(key)
	if err != nil {
		return nil, fmt.Errorf("cannot parse SSH key: %w", err)
	}

	sshCfg := &ssh.ClientConfig{
		User:            cfg.User,
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(signer)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // TODO: use known_hosts in production
		Timeout:         15 * time.Second,
	}

	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	conn, err := ssh.Dial("tcp", addr, sshCfg)
	if err != nil {
		return nil, fmt.Errorf("SSH connection to %s failed: %w", addr, err)
	}

	return &Client{conn: conn, cfg: cfg}, nil
}

// resolveSSHKey returns the PEM bytes for the SSH private key.
//
// Priority:
//  1. PILOT_SSH_KEY env var — PEM content as a string (for CI/CD).
//     GitHub Actions encodes secrets with literal \n; we normalise them.
//  2. keyPath file — expanded from ~/
func resolveSSHKey(keyPath string) ([]byte, error) {
	if raw := os.Getenv("PILOT_SSH_KEY"); raw != "" {
		// GitHub Actions stores multi-line secrets with literal \n sequences.
		// Normalize them to actual newlines so ssh.ParsePrivateKey works.
		raw = strings.ReplaceAll(raw, `\n`, "\n")
		return []byte(strings.TrimSpace(raw) + "\n"), nil
	}

	if keyPath == "" {
		return nil, fmt.Errorf(
			"no SSH key configured\n" +
				"  Set one of:\n" +
				"    targets.<name>.key in pilot.yaml  (e.g. ~/.ssh/id_pilot)\n" +
				"    export PILOT_SSH_KEY=\"$(cat ~/.ssh/id_pilot)\"  (for CI/CD)",
		)
	}

	if strings.HasPrefix(keyPath, "~/") {
		home, _ := os.UserHomeDir()
		keyPath = filepath.Join(home, keyPath[2:])
	}

	key, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, fmt.Errorf(
			"cannot read SSH key %s: %w\n"+
				"  Alternatives:\n"+
				"    ssh-keygen -t ed25519 -f %s\n"+
				"    export PILOT_SSH_KEY=\"$(cat %s)\"",
			keyPath, err, keyPath, keyPath,
		)
	}
	return key, nil
}

// Run executes a command on the remote host and returns combined output.
func (c *Client) Run(ctx context.Context, command string) (string, error) {
	session, err := c.conn.NewSession()
	if err != nil {
		return "", fmt.Errorf("new SSH session: %w", err)
	}
	defer session.Close()

	done := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			session.Close()
		case <-done:
		}
	}()
	defer close(done)

	out, err := session.CombinedOutput(command)
	return string(out), err
}

// CopyFiles uploads local files to a remote directory using SCP-over-SSH.
// All files land flat in remoteDir (basename only, no subdirectory structure).
func (c *Client) CopyFiles(ctx context.Context, localPaths []string, remoteDir string) error {
	if _, err := c.Run(ctx, fmt.Sprintf("mkdir -p %s", remoteDir)); err != nil {
		return fmt.Errorf("mkdir %s: %w", remoteDir, err)
	}
	for _, localPath := range localPaths {
		if err := c.scpFile(ctx, localPath, remoteDir); err != nil {
			return err
		}
	}
	return nil
}

// CopyFileTo uploads a single local file to an exact remote path,
// creating parent directories as needed.
// Use this instead of CopyFiles when the remote path must differ from the basename
// (e.g. preserving relative directory structure: ./nginx/prod.conf → ~/pilot/nginx/prod.conf).
func (c *Client) CopyFileTo(ctx context.Context, localPath, remotePath string) error {
	// Ensure the parent directory exists on the remote.
	remoteDir := filepath.Dir(remotePath)
	if _, err := c.Run(ctx, fmt.Sprintf("mkdir -p %s", remoteDir)); err != nil {
		return fmt.Errorf("mkdir %s: %w", remoteDir, err)
	}

	data, err := os.ReadFile(localPath)
	if err != nil {
		return fmt.Errorf("read %s: %w", localPath, err)
	}

	session, err := c.conn.NewSession()
	if err != nil {
		return fmt.Errorf("new SSH session: %w", err)
	}
	defer session.Close()

	filename := filepath.Base(remotePath)
	pipe, err := session.StdinPipe()
	if err != nil {
		return err
	}

	go func() {
		defer pipe.Close()
		fmt.Fprintf(pipe, "C0644 %d %s\n", len(data), filename)
		io.Copy(pipe, strings.NewReader(string(data)))
		fmt.Fprint(pipe, "\x00")
	}()

	return session.Run(fmt.Sprintf("scp -t %s", remoteDir))
}

func (c *Client) scpFile(ctx context.Context, localPath, remoteDir string) error {
	data, err := os.ReadFile(localPath)
	if err != nil {
		return fmt.Errorf("read %s: %w", localPath, err)
	}

	session, err := c.conn.NewSession()
	if err != nil {
		return fmt.Errorf("new SSH session: %w", err)
	}
	defer session.Close()

	filename := filepath.Base(localPath)
	remotePath := fmt.Sprintf("%s/%s", strings.TrimRight(remoteDir, "/"), filename)

	pipe, err := session.StdinPipe()
	if err != nil {
		return err
	}

	go func() {
		defer pipe.Close()
		fmt.Fprintf(pipe, "C0644 %d %s\n", len(data), filename)
		io.Copy(pipe, strings.NewReader(string(data)))
		fmt.Fprint(pipe, "\x00")
	}()

	_ = remotePath
	return session.Run(fmt.Sprintf("scp -t %s", remoteDir))
}

// RunWithOutput executes a command on the remote host and writes combined
// stdout+stderr to w in real time. Returns an error if the command exits
// non-zero. Unlike Run, the caller sees output as it arrives — useful for
// long-running commands such as docker pull or docker compose up.
func (c *Client) RunWithOutput(ctx context.Context, command string, w io.Writer) error {
	session, err := c.conn.NewSession()
	if err != nil {
		return fmt.Errorf("new SSH session: %w", err)
	}
	defer session.Close()

	session.Stdout = w
	session.Stderr = w

	done := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			session.Close()
		case <-done:
		}
	}()
	defer close(done)

	return session.Run(command)
}

// Stream executes a command on the remote host and streams output line by line.
// The returned channel is closed when the command exits or ctx is cancelled.
func (c *Client) Stream(ctx context.Context, command string) (<-chan string, error) {
	session, err := c.conn.NewSession()
	if err != nil {
		return nil, fmt.Errorf("new SSH session: %w", err)
	}

	pipe, err := session.StdoutPipe()
	if err != nil {
		session.Close()
		return nil, fmt.Errorf("stdout pipe: %w", err)
	}
	session.Stderr = io.Discard

	if err := session.Start(command); err != nil {
		session.Close()
		return nil, fmt.Errorf("start command: %w", err)
	}

	ch := make(chan string, 64)
	go func() {
		defer close(ch)
		defer session.Close()

		done := make(chan struct{})
		go func() {
			select {
			case <-ctx.Done():
				session.Close()
			case <-done:
			}
		}()
		defer close(done)

		buf := make([]byte, 4096)
		for {
			n, err := pipe.Read(buf)
			if n > 0 {
				for _, line := range strings.Split(string(buf[:n]), "\n") {
					if line != "" {
						ch <- line
					}
				}
			}
			if err != nil {
				break
			}
		}
		_ = session.Wait()
	}()

	return ch, nil
}

// Close terminates the SSH connection.
func (c *Client) Close() error {
	return c.conn.Close()
}
