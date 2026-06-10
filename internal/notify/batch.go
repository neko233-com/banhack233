package notify

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/neko233-com/banhack233/internal/config"
	"github.com/neko233-com/banhack233/internal/geoip"
)

type Dispatcher struct {
	cfg        config.NotificationSet
	batch      config.NotifyBatchConfig
	geo        *geoip.Lookup
	geoEnabled bool

	mu        sync.Mutex
	pending   []Event
	lastFlush time.Time
}

func NewDispatcher(notifications config.NotificationSet, geoCfg config.GeoIPConfig) *Dispatcher {
	d := &Dispatcher{
		cfg:        notifications,
		batch:      notifications.Batch,
		geoEnabled: geoCfg.Enabled,
	}
	if geoCfg.Enabled {
		d.geo = geoip.New(geoCfg.DBPath)
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
	if !d.geoEnabled || d.geo == nil || ev.IP == "" || ev.IP == "-" {
		return ev
	}
	loc, err := d.geo.Lookup(ev.IP)
	if err == nil {
		if text := loc.String(); text != "" {
			ev.Location = text
		}
	}
	return ev
}

func (d *Dispatcher) NotifyBan(ctx context.Context, ev Event) error {
	ev = d.enrich(ev)
	if !d.batch.Enabled {
		return Send(ctx, d.cfg, ev)
	}
	d.mu.Lock()
	d.pending = append(d.pending, ev)
	full := len(d.pending) >= d.batch.MaxItems
	d.mu.Unlock()
	if full {
		return d.Flush(ctx)
	}
	return nil
}

func (d *Dispatcher) NotifyAudit(ctx context.Context, ev Event) error {
	return Send(ctx, d.cfg, ev)
}

func (d *Dispatcher) FlushIfDue(ctx context.Context) error {
	if !d.batch.Enabled {
		return nil
	}
	d.mu.Lock()
	due := len(d.pending) > 0 && time.Since(d.lastFlush) >= d.batch.Interval.Duration
	d.mu.Unlock()
	if due {
		return d.Flush(ctx)
	}
	return nil
}

func (d *Dispatcher) Flush(ctx context.Context) error {
	if !d.batch.Enabled {
		return nil
	}
	d.mu.Lock()
	if len(d.pending) == 0 {
		d.mu.Unlock()
		return nil
	}
	items := append([]Event(nil), d.pending...)
	d.pending = nil
	d.lastFlush = time.Now()
	d.mu.Unlock()
	return sendBatch(ctx, d.cfg, items)
}

func sendBatch(ctx context.Context, cfg config.NotificationSet, items []Event) error {
	if len(items) == 0 {
		return nil
	}
	if len(items) == 1 {
		return Send(ctx, cfg, items[0])
	}
	alert := alertFromBanBatch(items)
	if cfg.Console {
		fmt.Println(alert.ConsoleText())
	}
	return sendAlert(ctx, cfg, alert)
}
