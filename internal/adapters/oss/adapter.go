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

// Package oss implements PlatformAdapter against the open-source Apache Flink
// REST API. It is read-only and holds no Flink-specific types in its public
// surface; all wire shapes are mapped into the Flinkit model.
package oss

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/smanjakuppam-at-s3i/flinkit/internal/adapters"
	"github.com/smanjakuppam-at-s3i/flinkit/internal/model"
)

const defaultEndpoint = "http://localhost:8081"

// Adapter talks to one Flink cluster's REST API.
type Adapter struct {
	baseURL string
	client  *http.Client
}

var _ adapters.PlatformAdapter = (*Adapter)(nil)

// init registers the adapter so `--source oss` resolves it through the default
// registry once the package is linked in.
func init() { adapters.Register("oss", New) }

// New builds an OSS adapter from a Config, defaulting to localhost:8081 over a
// plain HTTP client with a 10s timeout.
func New(cfg adapters.Config) (adapters.PlatformAdapter, error) {
	base := strings.TrimSpace(cfg.Endpoint)
	if base == "" {
		base = defaultEndpoint
	}
	return NewWithClient(base, &http.Client{Timeout: 10 * time.Second}), nil
}

// NewWithClient builds an OSS adapter that talks to baseURL over the given HTTP
// client. It exists so another adapter can reuse this REST implementation over a
// different transport — e.g. the k8s adapter reaching a JobManager through the
// Kubernetes apiserver proxy (whose client carries the cluster's auth and TLS)
// or in-cluster service DNS. A nil client falls back to a plain 10s-timeout
// client so callers only supplying a baseURL still get sane defaults.
func NewWithClient(baseURL string, client *http.Client) *Adapter {
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	return &Adapter{
		baseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		client:  client,
	}
}

// Name returns the adapter identifier.
func (a *Adapter) Name() string { return "oss" }

// ListJobs returns lightweight job metadata from /jobs/overview.
func (a *Adapter) ListJobs(ctx context.Context) ([]model.Job, error) {
	var dto jobsOverviewDTO
	if err := a.getJSON(ctx, "/jobs/overview", &dto); err != nil {
		return nil, err
	}
	jobs := make([]model.Job, 0, len(dto.Jobs))
	for _, j := range dto.Jobs {
		jobs = append(jobs, model.Job{
			ID:       j.JID,
			Name:     j.Name,
			Platform: model.PlatformOSS,
			State:    model.JobState(j.State),
		})
	}
	return jobs, nil
}

// GetJob returns one job's metadata without its operator graph.
func (a *Adapter) GetJob(ctx context.Context, jobID string) (model.Job, error) {
	d, err := a.jobDetail(ctx, jobID)
	if err != nil {
		return model.Job{}, err
	}
	return model.Job{
		ID:           d.JID,
		Name:         d.Name,
		Platform:     model.PlatformOSS,
		State:        model.JobState(d.State),
		FlinkVersion: model.ParseVersion(a.clusterVersion(ctx)),
	}, nil
}

// GetJobGraph returns the job with its operators and upstream edges, which the
// edges in the job plan supply.
func (a *Adapter) GetJobGraph(ctx context.Context, jobID string) (model.Job, error) {
	d, err := a.jobDetail(ctx, jobID)
	if err != nil {
		return model.Job{}, err
	}
	job := model.Job{
		ID:           d.JID,
		Name:         d.Name,
		Platform:     model.PlatformOSS,
		State:        model.JobState(d.State),
		FlinkVersion: model.ParseVersion(a.clusterVersion(ctx)),
	}
	upstreams := make(map[string][]string, len(d.Plan.Nodes))
	for _, n := range d.Plan.Nodes {
		for _, in := range n.Inputs {
			upstreams[n.ID] = append(upstreams[n.ID], in.ID)
		}
	}
	for _, v := range d.Vertices {
		job.Operators = append(job.Operators, model.Operator{
			ID:          v.ID,
			Name:        v.Name,
			Parallelism: v.Parallelism,
			Upstreams:   upstreams[v.ID],
		})
	}
	return job, nil
}

// GetMetrics returns job-scoped metrics: checkpoint completed/failed counts and
// the restart count. Fetching is best-effort — a metric whose endpoint is
// unavailable is simply omitted, so callers degrade gracefully rather than
// failing the whole diagnosis.
func (a *Adapter) GetMetrics(ctx context.Context, jobID string) (adapters.Metrics, error) {
	m := adapters.Metrics{}
	id := url.PathEscape(jobID)

	var cp checkpointsDTO
	if err := a.getJSON(ctx, "/jobs/"+id+"/checkpoints", &cp); err == nil {
		m["checkpoints.completed"] = float64(cp.Counts.Completed)
		m["checkpoints.failed"] = float64(cp.Counts.Failed)
	}

	var jm []jobMetricDTO
	if err := a.getJSON(ctx, "/jobs/"+id+"/metrics?get=numRestarts", &jm); err == nil {
		for _, entry := range jm {
			if entry.ID == "numRestarts" {
				if v, perr := strconv.ParseFloat(entry.Value, 64); perr == nil {
					m["numRestarts"] = v
				}
			}
		}
	}

	return m, nil
}

// GetSubtaskMetrics returns per-subtask metrics for one operator: record counts
// from the vertex detail, plus busy/backpressured/idle rates queried per
// subtask. Rate queries are best-effort — a subtask whose metrics endpoint is
// unavailable keeps its zero rates rather than failing the call.
func (a *Adapter) GetSubtaskMetrics(ctx context.Context, jobID, operatorID string) ([]model.Subtask, error) {
	base := "/jobs/" + url.PathEscape(jobID) + "/vertices/" + url.PathEscape(operatorID)

	var d vertexDetailDTO
	if err := a.getJSON(ctx, base, &d); err != nil {
		return nil, err
	}
	subs := make([]model.Subtask, 0, len(d.Subtasks))
	for _, s := range d.Subtasks {
		st := model.Subtask{
			Index:      s.Subtask,
			RecordsIn:  s.Metrics.ReadRecords,
			RecordsOut: s.Metrics.WriteRecords,
		}
		if r, err := a.subtaskRates(ctx, base, s.Subtask); err == nil {
			st.BusyMsPerSec = r.busy
			st.BackpressuredMsPerSec = r.backpressured
			st.IdleMsPerSec = r.idle
		}
		subs = append(subs, st)
	}
	return subs, nil
}

type subtaskRateSet struct {
	busy          float64
	backpressured float64
	idle          float64
}

// subtaskRates queries one subtask's busy/backpressured/idle per-second rates.
func (a *Adapter) subtaskRates(ctx context.Context, vertexBase string, idx int) (subtaskRateSet, error) {
	path := vertexBase + "/subtasks/" + strconv.Itoa(idx) +
		"/metrics?get=busyTimeMsPerSecond,backPressuredTimeMsPerSecond,idleTimeMsPerSecond"
	var metrics []jobMetricDTO
	if err := a.getJSON(ctx, path, &metrics); err != nil {
		return subtaskRateSet{}, err
	}
	var r subtaskRateSet
	for _, m := range metrics {
		v, err := strconv.ParseFloat(m.Value, 64)
		if err != nil {
			continue
		}
		switch m.ID {
		case "busyTimeMsPerSecond":
			r.busy = v
		case "backPressuredTimeMsPerSecond":
			r.backpressured = v
		case "idleTimeMsPerSecond":
			r.idle = v
		}
	}
	return r, nil
}

// GetConfig returns a job's effective user configuration.
func (a *Adapter) GetConfig(ctx context.Context, jobID string) (map[string]string, error) {
	var d jobConfigDTO
	if err := a.getJSON(ctx, "/jobs/"+url.PathEscape(jobID)+"/config", &d); err != nil {
		return nil, err
	}
	return d.ExecutionConfig.UserConfig, nil
}

// ListStatements returns nothing: the OSS REST API exposes no SQL statement
// registry. Statements are discovered from source by the repo adapter.
func (a *Adapter) ListStatements(ctx context.Context) ([]model.Statement, error) {
	return nil, nil
}

// Capabilities declares the platform. The matrix is filled in as rules need it.
func (a *Adapter) Capabilities() adapters.Capabilities {
	return adapters.Capabilities{Platform: model.PlatformOSS}
}

// GetLogs returns the last maxLines of the JobManager log. The JM log carries
// job lifecycle, restarts, checkpoint failures, and propagated exceptions —
// the most useful lines for triage. TaskManager logs are a later refinement.
// (Implements adapters.LogProvider.)
func (a *Adapter) GetLogs(ctx context.Context, jobID string, maxLines int) ([]adapters.LogLine, error) {
	text, err := a.getText(ctx, "/jobmanager/log")
	if err != nil {
		return nil, err
	}
	lines := strings.Split(strings.TrimRight(text, "\n"), "\n")
	lines = lastLines(lines, maxLines)
	out := make([]adapters.LogLine, 0, len(lines))
	for _, l := range lines {
		if strings.TrimSpace(l) == "" {
			continue
		}
		out = append(out, adapters.LogLine{Source: "jobmanager", Text: l})
	}
	return out, nil
}

func lastLines(lines []string, n int) []string {
	if n <= 0 || len(lines) <= n {
		return lines
	}
	return lines[len(lines)-n:]
}

// getText performs a GET and returns the body as text (for plain-text endpoints
// like the log files), capped to a sane size.
func (a *Adapter) getText(ctx context.Context, path string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, a.baseURL+path, nil)
	if err != nil {
		return "", fmt.Errorf("oss: building request for %s: %w", path, err)
	}
	resp, err := a.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("oss: GET %s: %w", path, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("oss: GET %s: unexpected status %d", path, resp.StatusCode)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // cap 1 MiB
	if err != nil {
		return "", fmt.Errorf("oss: reading %s: %w", path, err)
	}
	return string(b), nil
}

// clusterVersion returns the Flink version from the cluster /config endpoint,
// best-effort (empty on error — the REST job detail itself carries no version).
func (a *Adapter) clusterVersion(ctx context.Context) string {
	var c clusterConfigDTO
	if err := a.getJSON(ctx, "/config", &c); err != nil {
		return ""
	}
	return c.FlinkVersion
}

func (a *Adapter) jobDetail(ctx context.Context, jobID string) (jobDetailDTO, error) {
	var d jobDetailDTO
	err := a.getJSON(ctx, "/jobs/"+url.PathEscape(jobID), &d)
	return d, err
}

// getJSON performs a GET and decodes a JSON body into out.
func (a *Adapter) getJSON(ctx context.Context, path string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, a.baseURL+path, nil)
	if err != nil {
		return fmt.Errorf("oss: building request for %s: %w", path, err)
	}
	resp, err := a.client.Do(req)
	if err != nil {
		return fmt.Errorf("oss: GET %s: %w", path, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("oss: GET %s: unexpected status %d: %s", path, resp.StatusCode, strings.TrimSpace(string(body)))
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("oss: decoding %s: %w", path, err)
	}
	return nil
}
