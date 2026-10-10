package notify

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/neko233-com/banhack233/internal/config"
	"github.com/neko233-com/banhack233/internal/geoip"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type queuedAlert struct {
	ID       int64     `json:"id"`
	Alert    Alert     `json:"alert"`
	Pending  []string  `json:"pending"`
	Attempts int       `json:"attempts"`
	Created  time.Time `json:"created"`
	Next     time.Time `json:"next"`
}
type queueState struct {
	Pending   []Event       `json:"batch"`
	Alerts    []queuedAlert `json:"alerts"`
	LastFlush time.Time     `json:"last_flush"`
	Sequence  int64         `json:"sequence"`
}
type Dispatcher struct {
	cfg        config.NotificationSet
	batch      config.NotifyBatchConfig
	geo        *geoip.Lookup
	geoEnabled bool
	mu         sync.Mutex
	flushMu    sync.Mutex
	queue      queueState
	loadErr    error
}

func NewDispatcher(notifications config.NotificationSet, geoCfg config.GeoIPConfig) *Dispatcher {
	d := &Dispatcher{cfg: notifications, batch: notifications.Batch, geoEnabled: geoCfg.Enabled}
	if d.batch.MaxItems <= 0 {
		d.batch.MaxItems = 20
	}
	if d.batch.Interval.Duration <= 0 {
		d.batch.Interval = config.Duration{Duration: time.Minute}
	}
	if geoCfg.Enabled {
		d.geo = geoip.New(geoCfg.DBPath)
	}
	if notifications.QueuePath != "" {
		data, err := os.ReadFile(notifications.QueuePath)
		if err == nil {
			d.loadErr = json.Unmarshal(data, &d.queue)
		} else if !os.IsNotExist(err) {
			d.loadErr = err
		}
	}
	return d
}
func (d *Dispatcher) Close() error {
	if d.geo != nil {
		return d.geo.Close()
	}
	return nil
}
func (d *Dispatcher) enrich(ev Event) Event {
	if d.geoEnabled && d.geo != nil && ev.IP != "" && ev.IP != "-" {
		if loc, err := d.geo.Lookup(ev.IP); err == nil {
			ev.Location = loc.String()
		}
	}
	return ev
}
func (d *Dispatcher) persist() error {
	if d.loadErr != nil {
		return fmt.Errorf("notification queue cannot be loaded: %w", d.loadErr)
	}
	if d.cfg.QueuePath == "" {
		return nil
	}
	path := d.cfg.QueuePath
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	data, err := json.Marshal(d.queue)
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".notifications-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err = f.Write(data); err != nil {
		_ = f.Close()
		return err
	}
	if err = f.Sync(); err != nil {
		_ = f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}
func (d *Dispatcher) enqueue(a Alert) {
	ids := []string{}
	for _, c := range channels(d.cfg) {
		ids = append(ids, c.name)
	}
	if len(ids) == 0 {
		return
	}
	d.queue.Sequence++
	d.queue.Alerts = append(d.queue.Alerts, queuedAlert{ID: d.queue.Sequence, Alert: a, Pending: ids, Created: time.Now()})
}
func (d *Dispatcher) NotifyBan(_ context.Context, ev Event) error {
	ev = d.enrich(ev)
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.loadErr != nil {
		return d.loadErr
	}
	if len(d.queue.Alerts) >= 1000 {
		return fmt.Errorf("notification queue full (1000 alerts); inspect failed channels")
	}
	if !d.batch.Enabled || ev.Action == "notify" {
		d.enqueue(alertFromEvent(ev))
	} else {
		if len(d.queue.Pending) == 0 {
			d.queue.LastFlush = time.Now()
		}
		d.queue.Pending = append(d.queue.Pending, ev)
		if len(d.queue.Pending) >= d.batch.MaxItems {
			d.batchToQueue()
		}
	}
	return d.persist()
}
func (d *Dispatcher) NotifyAudit(_ context.Context, ev Event) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.loadErr != nil {
		return d.loadErr
	}
	if len(d.queue.Alerts) >= 1000 {
		return fmt.Errorf("notification queue full")
	}
	d.enqueue(alertFromEvent(ev))
	return d.persist()
}
func (d *Dispatcher) batchToQueue() {
	if len(d.queue.Pending) == 0 {
		return
	}
	if len(d.queue.Pending) == 1 {
		d.enqueue(alertFromEvent(d.queue.Pending[0]))
	} else {
		d.enqueue(alertFromBanBatch(d.queue.Pending))
	}
	d.queue.Pending = nil
	d.queue.LastFlush = time.Now()
}
func (d *Dispatcher) FlushIfDue(ctx context.Context) error { return d.flush(ctx, false) }
func (d *Dispatcher) Flush(ctx context.Context) error      { return d.flush(ctx, true) }
func (d *Dispatcher) flush(ctx context.Context, force bool) error {
	d.flushMu.Lock()
	defer d.flushMu.Unlock()
	d.mu.Lock()
	if d.loadErr != nil {
		d.mu.Unlock()
		return d.loadErr
	}
	if len(d.queue.Pending) > 0 && (force || time.Since(d.queue.LastFlush) >= d.batch.Interval.Duration) {
		d.batchToQueue()
		if err := d.persist(); err != nil {
			d.mu.Unlock()
			return err
		}
	}
	items := append([]queuedAlert(nil), d.queue.Alerts...)
	d.mu.Unlock()
	var errs []error
	for _, item := range items {
		if ctx.Err() != nil {
			return errors.Join(append(errs, ctx.Err())...)
		}
		if item.Next.After(time.Now()) {
			continue
		}
		var failed []string
		var err error
		if time.Since(item.Created) > 24*time.Hour {
			err = fmt.Errorf("notification %d expired after 24h", item.ID)
		} else {
			failed, err = deliver(ctx, d.cfg, item.Alert, item.Pending)
		}
		if err != nil {
			errs = append(errs, err)
		}
		d.mu.Lock()
		for i := range d.queue.Alerts {
			if d.queue.Alerts[i].ID != item.ID {
				continue
			}
			if len(failed) == 0 {
				d.queue.Alerts = append(d.queue.Alerts[:i], d.queue.Alerts[i+1:]...)
			} else {
				q := &d.queue.Alerts[i]
				q.Pending = failed
				q.Attempts++
				shift := q.Attempts - 1
				if shift > 6 {
					shift = 6
				}
				delay := time.Minute * time.Duration(1<<shift)
				if delay > time.Hour {
					delay = time.Hour
				}
				var deliveryErr *deliveryError
				if errors.As(err, &deliveryErr) && deliveryErr.RetryAfter > delay {
					delay = deliveryErr.RetryAfter
				}
				if delay > 24*time.Hour {
					delay = 24 * time.Hour
				}
				q.Next = time.Now().Add(delay)
			}
			break
		}
		saveErr := d.persist()
		d.mu.Unlock()
		if saveErr != nil {
			return errors.Join(append(errs, saveErr)...)
		}
	}
	return errors.Join(errs...)
}
func sendBatch(ctx context.Context, cfg config.NotificationSet, items []Event) error {
	if len(items) == 0 {
		return nil
	}
	if len(items) == 1 {
		return Send(ctx, cfg, items[0])
	}
	return sendAlert(ctx, cfg, alertFromBanBatch(items))
}
