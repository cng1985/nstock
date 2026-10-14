package monitor

import (
	"context"
	"sync"
	"time"

	"nstock/internal/config"
	"nstock/internal/service"
)

type Status struct {
	Running         bool       `json:"running"`
	IntervalSeconds int        `json:"intervalSeconds"`
	LastScan        *time.Time `json:"lastScan"`
	LastError       string     `json:"lastError"`
	Checked         int        `json:"checked"`
	Fired           int        `json:"fired"`
}

type Runner struct {
	svc      *service.Service
	interval time.Duration
	stop     chan struct{}
	mu       sync.Mutex
	running  bool
	last     time.Time
	lastErr  string
	checked  int
	fired    int
}

func New(svc *service.Service, cfg config.Config) *Runner {
	interval := cfg.MonitorInterval
	if interval <= 0 {
		interval = 20 * time.Second
	}
	return &Runner{svc: svc, interval: interval}
}

func (r *Runner) Start() {
	r.mu.Lock()
	if r.running {
		r.mu.Unlock()
		return
	}
	r.running = true
	r.stop = make(chan struct{})
	stop := r.stop
	r.mu.Unlock()
	go r.loop(stop)
}

func (r *Runner) Stop() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.running {
		return
	}
	r.running = false
	close(r.stop)
}

func (r *Runner) Scan(ctx context.Context) (service.ScanResult, error) {
	result, err := r.svc.Scan(ctx)
	r.record(result, err)
	return result, err
}

func (r *Runner) Status() Status {
	r.mu.Lock()
	defer r.mu.Unlock()
	status := Status{
		Running:         r.running,
		IntervalSeconds: int(r.interval / time.Second),
		LastError:       r.lastErr,
		Checked:         r.checked,
		Fired:           r.fired,
	}
	if !r.last.IsZero() {
		last := r.last
		status.LastScan = &last
	}
	return status
}

func (r *Runner) loop(stop chan struct{}) {
	timer := time.NewTimer(3 * time.Second)
	defer timer.Stop()
	for {
		select {
		case <-stop:
			return
		case <-timer.C:
			r.tick()
			timer.Reset(r.interval)
		}
	}
}

func (r *Runner) tick() {
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	_, _ = r.Scan(ctx)
}

func (r *Runner) record(result service.ScanResult, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.last = time.Now()
	r.checked = result.Checked
	r.fired = result.Fired
	if err != nil {
		r.lastErr = err.Error()
		return
	}
	r.lastErr = ""
}
