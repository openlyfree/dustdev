package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// podmanAPIVersion is the libpod REST API version prefix. Podman accepts any
// version it supports (>= 3.0); v5.0.0 covers the endpoints used here.
const podmanAPIVersion = "v5.0.0"

func containerName(slug string) string   { return "dustdev-" + slug }
func volumeName(projectID string) string { return "dustdev-proj-" + projectID }

type podmanClient struct {
	http *http.Client
	base string
}

func newPodmanClient(socket string) *podmanClient {
	return &podmanClient{
		http: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
					return (&net.Dialer{}).DialContext(ctx, "unix", socket)
				},
			},
		},
		base: "http://podman/" + podmanAPIVersion + "/libpod",
	}
}

type apiError struct {
	Status  int
	Message string
}

func (e *apiError) Error() string { return fmt.Sprintf("podman API %d: %s", e.Status, e.Message) }

func isNotFound(err error) bool {
	var ae *apiError
	if errors.As(err, &ae) {
		return ae.Status == http.StatusNotFound
	}
	return false
}

// do performs a libpod API call and treats any 2xx (plus extraOK) as success.
func (p *podmanClient) do(ctx context.Context, method, path string, body any, extraOK ...int) ([]byte, error) {
	var reader io.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reader = bytes.NewReader(payload)
	}

	req, err := http.NewRequestWithContext(ctx, method, p.base+path, reader)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := p.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("podman socket: %w", err)
	}
	defer resp.Body.Close()

	data, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return data, nil
	}
	for _, code := range extraOK {
		if resp.StatusCode == code {
			return data, nil
		}
	}

	// podman errors are JSON: {"message": "...", "response": <code>}
	var podErr struct {
		Message string `json:"message"`
	}
	if err := json.Unmarshal(data, &podErr); err == nil && podErr.Message != "" {
		return nil, &apiError{Status: resp.StatusCode, Message: podErr.Message}
	}
	return nil, &apiError{Status: resp.StatusCode, Message: strings.TrimSpace(string(data))}
}

type perNetworkOptions struct {
	Aliases []string `json:"aliases,omitempty"`
}

type linuxMemory struct {
	Limit int64 `json:"limit"`
}

type linuxCPU struct {
	Quota  int64  `json:"quota"`
	Period uint64 `json:"period"`
}

type linuxResources struct {
	Memory *linuxMemory `json:"memory,omitempty"`
	CPU    *linuxCPU    `json:"cpu,omitempty"`
}

type netNamespace struct {
	NSMode string `json:"nsmode"`
}

// namedVolume mirrors specgen.NamedVolume; the libpod API requires this object
// form (a plain "name:dest" string is rejected, and mounts with a named source
// silently degrade to bind mounts).
type namedVolume struct {
	Name string `json:"name"`
	Dest string `json:"dest"`
}

type createContainerSpec struct {
	Name           string                       `json:"name"`
	Image          string                       `json:"image"`
	Labels         map[string]string            `json:"labels,omitempty"`
	NetNS          netNamespace                 `json:"netns"`
	Networks       map[string]perNetworkOptions `json:"networks,omitempty"`
	Volumes        []namedVolume                `json:"volumes,omitempty"`
	ResourceLimits *linuxResources              `json:"resource_limits,omitempty"`
	// PullPolicy "always" makes start/restart fetch a fresh copy of the mutable
	// image tag (e.g. idehost:production), so CI-published IDE updates reach
	// projects without redeploying the platform itself.
	PullPolicy string `json:"pull_policy,omitempty"`
}

func (p *podmanClient) containerExists(ctx context.Context, name string) (bool, error) {
	_, err := p.do(ctx, http.MethodGet, "/containers/"+url.PathEscape(name)+"/exists", nil)
	if err == nil {
		return true, nil
	}
	if isNotFound(err) {
		return false, nil
	}
	return false, err
}

// ensureVolume creates the project's data volume if needed. Named volumes are
// auto-created by the podman CLI but not by the REST API, so do it explicitly.
func (p *podmanClient) ensureVolume(ctx context.Context, name string) error {
	_, err := p.do(ctx, http.MethodGet, "/volumes/"+url.PathEscape(name)+"/json", nil)
	if err == nil {
		return nil
	}
	if !isNotFound(err) {
		return err
	}
	_, err = p.do(ctx, http.MethodPost, "/volumes/create", map[string]string{"Name": name})
	return err
}

// ensureContainer creates the project's devcontainer if it does not exist yet.
// The container is created stopped; startContainer boots it.
func (p *podmanClient) ensureContainer(ctx context.Context, cfg config, proj project) error {
	if err := p.ensureVolume(ctx, volumeName(proj.ID)); err != nil {
		return fmt.Errorf("create volume: %w", err)
	}

	name := containerName(proj.Slug)
	exists, err := p.containerExists(ctx, name)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}

	spec := createContainerSpec{
		Name:       name,
		Image:      cfg.IDEImage,
		PullPolicy: "always",
		Labels: map[string]string{
			"dustdev.managed": "true",
			"dustdev.project": proj.ID,
			"dustdev.slug":    proj.Slug,
		},
		NetNS: netNamespace{NSMode: "bridge"},
		Networks: map[string]perNetworkOptions{
			cfg.PodmanNetwork: {Aliases: []string{proj.Slug}},
		},
		Volumes: []namedVolume{{Name: volumeName(proj.ID), Dest: "/workspace"}},
		ResourceLimits: &linuxResources{
			Memory: &linuxMemory{Limit: cfg.ContainerMemMB * 1024 * 1024},
			CPU:    &linuxCPU{Quota: int64(cfg.ContainerCPUs * 100000), Period: 100000},
		},
	}

	_, err = p.do(ctx, http.MethodPost, "/containers/create?name="+url.QueryEscape(name), spec)
	if err != nil && isNotFound(err) {
		return fmt.Errorf("image %q not found — pull it on the host (deploy/deploy.sh) or build it (make -C ide image)", cfg.IDEImage)
	}
	return err
}

func (p *podmanClient) startContainer(ctx context.Context, name string) error {
	// 304 = already running; idempotent start is fine.
	_, err := p.do(ctx, http.MethodPost, "/containers/"+url.PathEscape(name)+"/start", nil, http.StatusNotModified)
	return err
}

func (p *podmanClient) stopContainer(ctx context.Context, name string) error {
	_, err := p.do(ctx, http.MethodPost, "/containers/"+url.PathEscape(name)+"/stop?t=10", nil, http.StatusNotModified)
	if isNotFound(err) {
		return nil
	}
	return err
}

func (p *podmanClient) removeContainer(ctx context.Context, name string) error {
	_, err := p.do(ctx, http.MethodDelete, "/containers/"+url.PathEscape(name)+"?force=true", nil)
	if isNotFound(err) {
		return nil
	}
	return err
}

func (p *podmanClient) removeVolume(ctx context.Context, name string) error {
	_, err := p.do(ctx, http.MethodDelete, "/volumes/"+url.PathEscape(name)+"?force=true", nil)
	if isNotFound(err) {
		return nil
	}
	return err
}

type listedContainer struct {
	Names  []string          `json:"Names"`
	State  string            `json:"State"`
	Labels map[string]string `json:"Labels"`
}

// listManagedContainerStates returns container name -> state for every
// container carrying the dustdev.managed label (running or not).
func (p *podmanClient) listManagedContainerStates(ctx context.Context) (map[string]string, error) {
	filters := url.QueryEscape(`{"label":["dustdev.managed=true"]}`)
	data, err := p.do(ctx, http.MethodGet, "/containers/json?all=true&filters="+filters, nil)
	if err != nil {
		return nil, err
	}

	var containers []listedContainer
	if err := json.Unmarshal(data, &containers); err != nil {
		return nil, fmt.Errorf("decode container list: %w", err)
	}

	states := make(map[string]string, len(containers))
	for _, c := range containers {
		for _, n := range c.Names {
			states[strings.TrimPrefix(n, "/")] = c.State
		}
	}
	return states, nil
}
