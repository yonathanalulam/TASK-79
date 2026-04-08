package scheduler

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"fleetcommerce/internal/alerts"
	"fleetcommerce/internal/notifications"
	"fleetcommerce/internal/orders"
)

type Scheduler struct {
	orderSvc  *orders.Service
	alertSvc  *alerts.Service
	notifSvc  *notifications.Service
	cutoffInt time.Duration
	alertInt  time.Duration
	exportInt time.Duration
	stop      chan struct{}
	wg        sync.WaitGroup
}

func New(orderSvc *orders.Service, alertSvc *alerts.Service, notifSvc *notifications.Service,
	cutoffSeconds, alertSeconds, exportSeconds int) *Scheduler {
	return &Scheduler{
		orderSvc:  orderSvc,
		alertSvc:  alertSvc,
		notifSvc:  notifSvc,
		cutoffInt: time.Duration(cutoffSeconds) * time.Second,
		alertInt:  time.Duration(alertSeconds) * time.Second,
		exportInt: time.Duration(exportSeconds) * time.Second,
		stop:      make(chan struct{}),
	}
}

func (s *Scheduler) Start() {
	slog.Info("scheduler started", "cutoff_interval", s.cutoffInt, "alert_interval", s.alertInt, "export_interval", s.exportInt)

	s.wg.Add(3)
	go s.runJob("order-cutoff", s.cutoffInt, s.cutoffJob)
	go s.runJob("alert-evaluation", s.alertInt, s.alertJob)
	go s.runJob("export-retry", s.exportInt, s.exportJob)
}

func (s *Scheduler) Stop() {
	close(s.stop)
	s.wg.Wait()
	slog.Info("scheduler stopped")
}

func (s *Scheduler) runJob(name string, interval time.Duration, fn func(ctx context.Context)) {
	defer s.wg.Done()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-s.stop:
			return
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			slog.Debug("scheduler job starting", "job", name)
			fn(ctx)
			cancel()
		}
	}
}

func (s *Scheduler) cutoffJob(ctx context.Context) {
	count, err := s.orderSvc.ProcessCutoffs(ctx, 30) // 30 minute cutoff
	if err != nil {
		slog.Error("cutoff job failed", "error", err)
		return
	}
	if count > 0 {
		slog.Info("cutoff job completed", "orders_cutoff", count)
	}
}

func (s *Scheduler) alertJob(ctx context.Context) {
	count, err := s.alertSvc.EvaluateAlerts(ctx)
	if err != nil {
		slog.Error("alert evaluation failed", "error", err)
		return
	}
	if count > 0 {
		slog.Info("alert evaluation completed", "alerts_created", count)
	}
}

func (s *Scheduler) exportJob(ctx context.Context) {
	count, err := s.notifSvc.ProcessExportRetries(ctx)
	if err != nil {
		slog.Error("export retry failed", "error", err)
		return
	}
	if count > 0 {
		slog.Info("export retry completed", "items_processed", count)
	}
}
