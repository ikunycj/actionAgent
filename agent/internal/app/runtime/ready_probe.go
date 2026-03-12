package runtime

import "sync/atomic"

type ReadyProbe struct{ ready atomic.Bool }

func (r *ReadyProbe) Set(v bool)    { r.ready.Store(v) }
func (r *ReadyProbe) IsReady() bool { return r.ready.Load() }
