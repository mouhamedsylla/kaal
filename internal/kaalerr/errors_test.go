package kaalerr_test

import (
	"errors"
	"testing"

	"github.com/mouhamedsylla/kaal/internal/kaalerr"
)

func TestDeployError_ExitCode(t *testing.T) {
	err := &kaalerr.DeployError{Phase: "pull", Target: "vps-prod", Cause: errors.New("timeout")}
	if kaalerr.ExitCode(err) != kaalerr.ExitDeploy {
		t.Errorf("ExitCode = %d, want %d", kaalerr.ExitCode(err), kaalerr.ExitDeploy)
	}
}

func TestConfigError_ExitCode(t *testing.T) {
	err := &kaalerr.ConfigError{Path: "kaal.yaml", Cause: errors.New("invalid")}
	if kaalerr.ExitCode(err) != kaalerr.ExitConfig {
		t.Errorf("ExitCode = %d, want %d", kaalerr.ExitCode(err), kaalerr.ExitConfig)
	}
}

func TestSecretsError_ExitCode(t *testing.T) {
	err := &kaalerr.SecretsError{Provider: "aws_sm", Key: "db-url", Cause: errors.New("not found")}
	if kaalerr.ExitCode(err) != kaalerr.ExitSecrets {
		t.Errorf("ExitCode = %d, want %d", kaalerr.ExitCode(err), kaalerr.ExitSecrets)
	}
}

func TestSSHError_ExitCode(t *testing.T) {
	err := &kaalerr.SSHError{Host: "1.2.3.4", Op: "connect", Cause: errors.New("refused")}
	if kaalerr.ExitCode(err) != kaalerr.ExitSSH {
		t.Errorf("ExitCode = %d, want %d", kaalerr.ExitCode(err), kaalerr.ExitSSH)
	}
}

func TestExitCode_UnknownError(t *testing.T) {
	if kaalerr.ExitCode(errors.New("generic")) != kaalerr.ExitGeneral {
		t.Errorf("unknown error should return ExitGeneral")
	}
}

func TestExitCode_Nil(t *testing.T) {
	if kaalerr.ExitCode(nil) != kaalerr.ExitOK {
		t.Errorf("nil error should return ExitOK")
	}
}

func TestErrorMessages(t *testing.T) {
	cases := []struct {
		err  error
		want string
	}{
		{
			&kaalerr.DeployError{Phase: "pull", Target: "vps-prod", Cause: errors.New("timeout")},
			"deploy [pull] failed on vps-prod",
		},
		{
			&kaalerr.ConfigError{Path: "kaal.yaml", Cause: errors.New("invalid yaml")},
			"config kaal.yaml: invalid yaml",
		},
		{
			&kaalerr.SecretsError{Provider: "local", Key: "DB_URL", Cause: errors.New("not found")},
			"secrets [local]: key \"DB_URL\": not found",
		},
		{
			&kaalerr.SSHError{Host: "1.2.3.4", Op: "connect", Cause: errors.New("refused")},
			"ssh [1.2.3.4] connect: refused",
		},
	}

	for _, tc := range cases {
		if msg := tc.err.Error(); len(msg) == 0 {
			t.Errorf("Error() returned empty string for %T", tc.err)
		}
		// Check prefix matches expected
		if len(tc.err.Error()) < len(tc.want) || tc.err.Error()[:len(tc.want)] != tc.want {
			t.Errorf("%T.Error() = %q, want prefix %q", tc.err, tc.err.Error(), tc.want)
		}
	}
}

func TestErrorsUnwrap(t *testing.T) {
	cause := errors.New("root cause")
	err := &kaalerr.DeployError{Cause: cause}
	if !errors.Is(err, cause) {
		t.Error("errors.Is should find root cause via Unwrap")
	}
}
