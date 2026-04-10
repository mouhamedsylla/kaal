// Package plan defines the execution plan and compensation logic for pilot operations.
//
// A Plan is built from pilot.lock and describes which of the 7 execution nodes
// are active for a given operation, and how to roll them back if something fails.
//
// Compensation rules:
//   - LIFO order — the last executed step is rolled back first.
//   - reversible: false is the safe default. A step is only compensable when
//     explicitly marked reversible: true with a tested rollback_command.
//   - Only steps that actually ran are compensated (CompensationFor).
package plan

// StepName identifies one of the 7 execution nodes in the pilot skeleton.
type StepName string

const (
	StepPreflight   StepName = "preflight"
	StepPreHooks    StepName = "pre_hooks"
	StepMigrations  StepName = "migrations"
	StepPush        StepName = "push"
	StepDeploy      StepName = "deploy"
	StepPostHooks   StepName = "post_hooks"
	StepHealthcheck StepName = "healthcheck"
)

// Step is one node in the execution skeleton.
type Step struct {
	Name            StepName
	Active          bool   // false → skipped silently
	Reversible      bool   // default false — must be declared explicitly
	RollbackCommand string // required when Reversible is true
}

// Plan is the ordered list of steps for one pilot operation.
type Plan struct {
	Steps []Step
}

// ActiveSteps returns only the steps that are enabled for this execution.
func (p *Plan) ActiveSteps() []Step {
	var out []Step
	for _, s := range p.Steps {
		if s.Active {
			out = append(out, s)
		}
	}
	return out
}

// CompensationSteps returns all reversible steps in LIFO order.
// Use this when every step in the plan has run (e.g. healthcheck failure).
func (p *Plan) CompensationSteps() []Step {
	return compensate(p.Steps)
}

// CompensationFor returns the compensation plan for a failure at failedStep.
// It includes failedStep itself (if reversible) and all prior reversible steps,
// in LIFO order. Steps after failedStep are excluded — they never ran.
func (p *Plan) CompensationFor(failedStep StepName) []Step {
	// Collect steps up to and including the failed one.
	var ran []Step
	for _, s := range p.Steps {
		ran = append(ran, s)
		if s.Name == failedStep {
			break
		}
	}
	return compensate(ran)
}

// compensate returns the reversible subset of steps in reverse order.
func compensate(steps []Step) []Step {
	var out []Step
	for i := len(steps) - 1; i >= 0; i-- {
		if steps[i].Reversible {
			out = append(out, steps[i])
		}
	}
	return out
}
