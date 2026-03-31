package piloterr_test

import (
	"errors"
	"testing"

	"github.com/mouhamedsylla/pilot/internal/piloterr"
)

func TestDeployError_ExitCode(t *testing.T) {
	err := &piloterr.DeployError{Phase: "pull", Target: "vps-prod", Cause: errors.New("timeout")}
	if piloterr.ExitCode(err) != piloterr.ExitDeploy {
		t.Errorf("ExitCode = %d, want %d", piloterr.ExitCode(err), piloterr.ExitDeploy)
	}
}

func TestConfigError_ExitCode(t *testing.T) {
	err := &piloterr.ConfigError{Path: "pilot.yaml", Cause: errors.New("invalid")}
	if piloterr.ExitCode(err) != piloterr.ExitConfig {
		t.Errorf("ExitCode = %d, want %d", piloterr.ExitCode(err), piloterr.ExitConfig)
	}
}

func TestSecretsError_ExitCode(t *testing.T) {
	err := &piloterr.SecretsError{Provider: "aws_sm", Key: "db-url", Cause: errors.New("not found")}
	if piloterr.ExitCode(err) != piloterr.ExitSecrets {
		t.Errorf("ExitCode = %d, want %d", piloterr.ExitCode(err), piloterr.ExitSecrets)
	}
}

func TestSSHError_ExitCode(t *testing.T) {
	err := &piloterr.SSHError{Host: "1.2.3.4", Op: "connect", Cause: errors.New("refused")}
	if piloterr.ExitCode(err) != piloterr.ExitSSH {
		t.Errorf("ExitCode = %d, want %d", piloterr.ExitCode(err), piloterr.ExitSSH)
	}
}

func TestExitCode_UnknownError(t *testing.T) {
	if piloterr.ExitCode(errors.New("generic")) != piloterr.ExitGeneral {
		t.Errorf("unknown error should return ExitGeneral")
	}
}

func TestExitCode_Nil(t *testing.T) {
	if piloterr.ExitCode(nil) != piloterr.ExitOK {
		t.Errorf("nil error should return ExitOK")
	}
}

func TestErrorMessages(t *testing.T) {
	cases := []struct {
		err  error
		want string
	}{
		{
			&piloterr.DeployError{Phase: "pull", Target: "vps-prod", Cause: errors.New("timeout")},
			"deploy [pull] failed on vps-prod",
		},
		{
			&piloterr.ConfigError{Path: "pilot.yaml", Cause: errors.New("invalid yaml")},
			"config pilot.yaml: invalid yaml",
		},
		{
			&piloterr.SecretsError{Provider: "local", Key: "DB_URL", Cause: errors.New("not found")},
			"secrets [local]: key \"DB_URL\": not found",
		},
		{
			&piloterr.SSHError{Host: "1.2.3.4", Op: "connect", Cause: errors.New("refused")},
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
	err := &piloterr.DeployError{Cause: cause}
	if !errors.Is(err, cause) {
		t.Error("errors.Is should find root cause via Unwrap")
	}
}
