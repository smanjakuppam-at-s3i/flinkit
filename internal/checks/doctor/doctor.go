// Copyright 2026 S3I Groups LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package doctor holds the Go-backed doctor checks. Each registers itself with
// the default check registry under its rule ID; the rule metadata (severity,
// message, fix, docs) lives in the embedded YAML, so these functions only
// decide whether a job trips the rule and point at what tripped it.
package doctor

import (
	"fmt"

	"github.com/smanjakuppam-at-s3i/flinkit/internal/model"
	"github.com/smanjakuppam-at-s3i/flinkit/internal/rules"
)

// backpressureThreshold is the per-subtask backpressure rate (ms/s) above which
// DOC-BP-001 fires.
const backpressureThreshold = 500

func init() {
	rules.RegisterCheck(missingOperatorUID())
	rules.RegisterCheck(stateWithoutTTL())
	rules.RegisterCheck(failedCheckpoints())
	rules.RegisterCheck(excessiveRestarts())
	rules.RegisterCheck(sustainedBackpressure())
}

// target names the operator a finding is about, as job/operator.
func target(job model.Job, op model.Operator) string {
	return fmt.Sprintf("%s/%s", job.ID, op.ID)
}

// missingOperatorUID implements DOC-HYG-001: a *stateful* operator without a
// stable UID breaks savepoint restore. It only flags operators that have state,
// which also avoids a false positive on sources (like the Flink REST API) that
// don't expose operator UIDs at all — absent state evidence, it stays silent
// rather than claiming a missing uid it can't actually see.
func missingOperatorUID() rules.Check {
	return rules.CheckFunc{
		RuleID: "DOC-HYG-001",
		Fn: func(rule rules.Rule, ctx rules.EvalContext) rules.Findings {
			var out rules.Findings
			for _, op := range ctx.Job.Operators {
				if op.UID == "" && len(op.State) > 0 {
					out = append(out, rule.Finding(target(ctx.Job, op),
						fmt.Sprintf("stateful operator %q has no uid", op.Name)))
				}
			}
			return out
		},
	}
}

// stateWithoutTTL implements DOC-STATE-001: keyed state with no TTL grows
// without bound.
func stateWithoutTTL() rules.Check {
	return rules.CheckFunc{
		RuleID: "DOC-STATE-001",
		Fn: func(rule rules.Rule, ctx rules.EvalContext) rules.Findings {
			var out rules.Findings
			for _, op := range ctx.Job.Operators {
				for _, s := range op.State {
					if s.TTL == "" {
						out = append(out, rule.Finding(target(ctx.Job, op),
							fmt.Sprintf("state %q has no TTL", s.Name)))
					}
				}
			}
			return out
		},
	}
}

// failedCheckpoints implements DOC-CKPT-001: any failed checkpoints. Reads the
// live metric; silent when metrics are absent (offline).
func failedCheckpoints() rules.Check {
	return rules.CheckFunc{
		RuleID: "DOC-CKPT-001",
		Fn: func(rule rules.Rule, ctx rules.EvalContext) rules.Findings {
			failed, ok := ctx.Metrics["checkpoints.failed"]
			if !ok || failed <= 0 {
				return nil
			}
			return rules.Findings{rule.Finding(ctx.Job.ID,
				fmt.Sprintf("%d checkpoint(s) have failed", int64(failed)))}
		},
	}
}

// sustainedBackpressure implements DOC-BP-001: an operator with subtasks
// backpressured above the threshold. It reads the per-subtask rates (sampled
// live), so it is silent when they are absent, and its fix points at
// bottleneck for the root cause.
func sustainedBackpressure() rules.Check {
	return rules.CheckFunc{
		RuleID: "DOC-BP-001",
		Fn: func(rule rules.Rule, ctx rules.EvalContext) rules.Findings {
			var out rules.Findings
			for _, op := range ctx.Job.Operators {
				maxBP, worst := 0.0, -1
				for _, s := range op.Subtasks {
					if s.BackpressuredMsPerSec > maxBP {
						maxBP, worst = s.BackpressuredMsPerSec, s.Index
					}
				}
				if maxBP >= backpressureThreshold {
					out = append(out, rule.Finding(target(ctx.Job, op),
						fmt.Sprintf("subtask %d backpressured %.0f ms/s", worst, maxBP)))
				}
			}
			return out
		},
	}
}

// excessiveRestarts implements DOC-STAB-001: the job has restarted. The simple
// "> 0" threshold will move into rule data once the declarative matcher lands.
func excessiveRestarts() rules.Check {
	return rules.CheckFunc{
		RuleID: "DOC-STAB-001",
		Fn: func(rule rules.Rule, ctx rules.EvalContext) rules.Findings {
			restarts, ok := ctx.Metrics["numRestarts"]
			if !ok || restarts <= 0 {
				return nil
			}
			return rules.Findings{rule.Finding(ctx.Job.ID,
				fmt.Sprintf("job has restarted %d time(s)", int64(restarts)))}
		},
	}
}
