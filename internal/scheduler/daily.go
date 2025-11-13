package scheduler

import (
	"context"
	"errors"
	"time"
)

// Daily запускает callback один раз в сутки в заданное время.
type Daily struct {
	hour     int
	minute   int
	location *time.Location
	now      func() time.Time
	wait     func(context.Context, time.Duration) error
}

// NewDaily создает ежедневный планировщик на заданное время суток.
func NewDaily(hour, minute int, location *time.Location) (*Daily, error) {
	if hour < 0 || hour > 23 {
		return nil, errors.New("hour must be between 0 and 23")
	}
	if minute < 0 || minute > 59 {
		return nil, errors.New("minute must be between 0 and 59")
	}
	if location == nil {
		location = time.Local
	}

	scheduler := &Daily{
		hour:     hour,
		minute:   minute,
		location: location,
		now:      time.Now,
	}
	scheduler.wait = scheduler.waitWithContext

	return scheduler, nil
}

// Start запускает фоновую горутину, которая вызывает callback каждый день.
func (d *Daily) Start(ctx context.Context, callback func(context.Context)) {
	go d.run(ctx, callback)
}

func (d *Daily) run(ctx context.Context, callback func(context.Context)) {
	for {
		now := d.now().In(d.location)
		next := d.nextOccurrence(now)
		wait := next.Sub(now)

		if err := d.wait(ctx, wait); err != nil {
			return
		}

		callback(ctx)
	}
}

func (d *Daily) nextOccurrence(from time.Time) time.Time {
	target := time.Date(
		from.Year(),
		from.Month(),
		from.Day(),
		d.hour,
		d.minute,
		0,
		0,
		d.location,
	)

	if !target.After(from) {
		target = target.Add(24 * time.Hour)
	}

	return target
}

func (d *Daily) waitWithContext(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
