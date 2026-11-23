package memstore

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/mio-cc/anywhere-door/engine/store"
)

type checkpoints struct{ s *memStore }

func (r checkpoints) Put(ctx context.Context, cp store.Checkpoint, keepHistory bool) error {
	return r.s.write(func(d *data) error {
		b, err := enc(cp)
		if err != nil {
			return err
		}
		d.cps[key(cp.RunID, cp.ExecKey)] = b
		if keepHistory {
			d.cpHist[key(cp.RunID, cp.ExecKey, num(cp.Seq))] = b
		}
		return nil
	})
}

func (r checkpoints) Latest(ctx context.Context, runID, execKey string) (cp store.Checkpoint, ok bool, err error) {
	err = r.s.read(func(d *data) error {
		b, found := d.cps[key(runID, execKey)]
		if !found {
			return nil
		}
		cp, err = dec[store.Checkpoint](b)
		ok = err == nil
		return err
	})
	return cp, ok, err
}

func (r checkpoints) At(ctx context.Context, runID, execKey string, seq int64) (cp store.Checkpoint, ok bool, err error) {
	err = r.s.read(func(d *data) error {
		pre := prefix(runID, execKey)
		best := int64(-1)
		for k := range d.cpHist {
			if n := lastNum(k); len(k) > len(pre) && k[:len(pre)] == pre && n <= seq && n > best {
				best = n
			}
		}
		if best < 0 {
			return nil
		}
		cp, err = dec[store.Checkpoint](d.cpHist[key(runID, execKey, num(best))])
		ok = err == nil
		return err
	})
	return cp, ok, err
}

type toolCalls struct{ s *memStore }

func (r toolCalls) Begin(ctx context.Context, c store.ToolCall) (existing *store.ToolCall, err error) {
	err = r.s.write(func(d *data) error {
		if b, ok := d.tools[c.IdemKey]; ok {
			old, err := dec[store.ToolCall](b)
			existing = &old
			return err
		}
		if c.Status == "" {
			c.Status = store.ToolStarted
		}
		b, err := enc(c)
		if err == nil {
			d.tools[c.IdemKey] = b
		}
		return err
	})
	return existing, err
}

func (r toolCalls) Finish(ctx context.Context, idemKey, status, resultRef string, endedAt time.Time) error {
	return r.s.write(func(d *data) error {
		b, ok := d.tools[idemKey]
		if !ok {
			return store.ErrNotFound
		}
		c, err := dec[store.ToolCall](b)
		if err != nil {
			return err
		}
		if c.Status != store.ToolStarted || (status != store.ToolSucceeded && status != store.ToolFailed) {
			return fmt.Errorf("%w：工具调用不能从 %s 改成 %s", store.ErrConflict, c.Status, status)
		}
		end := endedAt.UTC()
		c.Status, c.ResultRef, c.EndedAt = status, resultRef, &end
		if b, err = enc(c); err == nil {
			d.tools[idemKey] = b
		}
		return err
	})
}

func (r toolCalls) Get(ctx context.Context, idemKey string) (c store.ToolCall, err error) {
	err = r.s.read(func(d *data) error {
		b, ok := d.tools[idemKey]
		if !ok {
			return store.ErrNotFound
		}
		c, err = dec[store.ToolCall](b)
		return err
	})
	return c, err
}

type settings struct{ s *memStore }

func (r settings) Get(ctx context.Context, k string) (v json.RawMessage, ok bool, err error) {
	err = r.s.read(func(d *data) error {
		b, found := d.settings[k]
		if found {
			v, ok = append(json.RawMessage{}, b...), true
		}
		return nil
	})
	return v, ok, err
}

func (r settings) Put(ctx context.Context, k string, value json.RawMessage, at time.Time) error {
	if !json.Valid(value) {
		return fmt.Errorf("store: 设置 %s 的值不是合法的 JSON", k)
	}
	return r.s.write(func(d *data) error { d.settings[k] = append([]byte{}, value...); return nil })
}

func (r settings) All(ctx context.Context) (out map[string]json.RawMessage, err error) {
	err = r.s.read(func(d *data) error {
		out = make(map[string]json.RawMessage, len(d.settings))
		for k, b := range d.settings {
			out[k] = append(json.RawMessage{}, b...)
		}
		return nil
	})
	return out, err
}
