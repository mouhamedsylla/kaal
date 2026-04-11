// Package planview implements the "pilot plan" use case.
//
// It reads pilot.lock and renders the execution plan for a given operation
// without executing anything. The output includes:
//   - ordered active steps
//   - migration tool + command (if any)
//   - compensation plan (what would be rolled back in LIFO order on failure)
//   - whether pilot.lock is up to date
package planview

import (
	"fmt"

	"github.com/mouhamedsylla/pilot/internal/domain/lock"
	"github.com/mouhamedsylla/pilot/internal/domain/plan"
)

// Input selects which plan to display.
type Input struct {
	Operation  string // "deploy" (only one for now)
	ProjectDir string // directory containing pilot.lock (usually ".")
	Env        string
}

// StepInfo is one line of the plan.
type StepInfo struct {
	Name        string
	Description string
	Reversible  bool
}

// CompensationStep is one line of the compensation plan.
type CompensationStep struct {
	Step    string
	Command string
}

// Output is the full plan display.
type Output struct {
	Operation    string
	Env          string
	Steps        []StepInfo
	Compensation []CompensationStep
	LockFresh    bool   // false = stale or not found
	LockWarning  string // non-empty when LockFresh=false
}

// UseCase builds the plan view from pilot.lock.
type UseCase struct{}

func New() *UseCase { return &UseCase{} }

// Execute loads pilot.lock and returns the human-readable plan.
func (uc *UseCase) Execute(in Input) (Output, error) {
	if in.Operation == "" {
		in.Operation = "deploy"
	}
	if in.ProjectDir == "" {
		in.ProjectDir = "."
	}

	out := Output{Operation: in.Operation, Env: in.Env, LockFresh: true}

	lck, err := lock.Read(in.ProjectDir)
	if err != nil {
		out.LockFresh = false
		out.LockWarning = fmt.Sprintf("%v\n  Run: pilot preflight --target deploy", err)
		// Return a default plan so the user still sees *something*.
		out.Steps = defaultPlan()
		return out, nil
	}

	out.Steps = buildSteps(lck)
	out.Compensation = buildCompensation(lck)

	return out, nil
}

func buildSteps(lck *lock.Lock) []StepInfo {
	activeNodes := lck.ActiveNodes()
	skeleton := []struct {
		name plan.StepName
		desc string
	}{
		{plan.StepPreflight, "verify config, secrets, SSH reachability"},
		{plan.StepPreHooks, "run pre-deploy hooks on remote"},
		{plan.StepMigrations, migrationDesc(lck)},
		{plan.StepDeploy, fmt.Sprintf("pull image + docker compose up  [provider: %s]", lck.ExecutionProvider)},
		{plan.StepPostHooks, "run post-deploy hooks on remote"},
		{plan.StepHealthcheck, "wait for all services healthy"},
	}

	var steps []StepInfo
	for _, s := range skeleton {
		if !activeNodes[s.name] {
			continue
		}
		steps = append(steps, StepInfo{
			Name:        string(s.name),
			Description: s.desc,
			Reversible:  isReversible(s.name, lck),
		})
	}
	return steps
}

func buildCompensation(lck *lock.Lock) []CompensationStep {
	var out []CompensationStep
	activeNodes := lck.ActiveNodes()

	// LIFO: deploy → migrations (only compensable ones)
	if activeNodes[plan.StepDeploy] {
		out = append(out, CompensationStep{
			Step:    string(plan.StepDeploy),
			Command: "restore previous image tag",
		})
	}
	if activeNodes[plan.StepMigrations] && lck.ExecutionPlan.Migrations != nil && lck.ExecutionPlan.Migrations.Reversible {
		out = append(out, CompensationStep{
			Step:    string(plan.StepMigrations),
			Command: lck.ExecutionPlan.Migrations.RollbackCommand,
		})
	}
	return out
}

func migrationDesc(lck *lock.Lock) string {
	m := lck.ExecutionPlan.Migrations
	if m == nil {
		return "run database migrations"
	}
	rev := "irreversible"
	if m.Reversible {
		rev = "reversible"
	}
	return fmt.Sprintf("run %s migrations (%s) — %s", m.Tool, m.Command, rev)
}

func isReversible(name plan.StepName, lck *lock.Lock) bool {
	switch name {
	case plan.StepDeploy:
		return true // always — previous image is known
	case plan.StepMigrations:
		return lck.ExecutionPlan.Migrations != nil && lck.ExecutionPlan.Migrations.Reversible
	default:
		return false
	}
}

func defaultPlan() []StepInfo {
	return []StepInfo{
		{Name: string(plan.StepPreflight), Description: "verify config, secrets, SSH reachability"},
		{Name: string(plan.StepDeploy), Description: "pull image + docker compose up", Reversible: true},
		{Name: string(plan.StepHealthcheck), Description: "wait for all services healthy"},
	}
}
