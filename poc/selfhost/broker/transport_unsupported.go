//go:build !linux && !darwin

package broker

import "context"

func Serve(context.Context, Config, func()) error           { return ErrBroker }
func (Client) Call(context.Context, Request) (Reply, error) { return Reply{}, ErrBroker }
