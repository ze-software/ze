// Design: docs/architecture/container/network.md -- the participant: the link at the created call, its removal at the ended call
// Related: apply.go -- the plans the participant follows
// Related: namespace_linux.go -- the namespace inode the link request carries

package network

import (
	"context"
	"errors"
	"time"

	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/internal/plugins/container"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// participantName names the network package in a container's state when one
// of its calls fails, and in the config sibling registry.
const participantName = "network"

// cleanupTimeout bounds the removal of the links a failed created call made.
// It runs on a context of its own, because the failure may be the created
// call's own deadline.
const cleanupTimeout = 5 * time.Second

// participant is the network package's part in every container's start and
// end. Safe for concurrent use: it holds the manager, which locks.
type participant struct {
	manager *manager
}

// Name implements container.Participant.
func (p *participant) Name() string {
	return participantName
}

// Contribute implements container.Participant. The network package mounts
// nothing in this slice: the resolver file is the name resolution part of
// the network design, which is not built yet.
func (p *participant) Contribute(_ context.Context, _ string) ([]container.FileMount, error) {
	return []container.FileMount{}, nil
}

// Created implements container.Participant: it makes the instance's links.
func (p *participant) Created(ctx context.Context, call container.CreatedCall) error {
	return p.manager.created(ctx, call)
}

// Ended implements container.Participant: it removes the instance's links.
func (p *participant) Ended(ctx context.Context, instance string) {
	p.manager.ended(ctx, instance)
}

// created makes one link per attachment of the instance, inside the created
// call's deadline. It first removes any link still recorded for the instance,
// which a start the deadline aborted may have left. On a failure it removes
// what it made and answers the failure, which aborts the start. An instance
// with no attachment returns at once.
func (m *manager) created(ctx context.Context, call container.CreatedCall) error {
	m.mu.Lock()
	plans := m.plans[call.Instance]
	client := m.client
	m.mu.Unlock()

	if len(plans) == 0 {
		return nil
	}
	if client == nil {
		return refuseNoEngine("container " + call.Instance + " was not attached to its networks")
	}
	if _, err := client.OwnedLinkRemove(ctx, &rpc.OwnedLinkRemoveInput{Key: call.Instance}); err != nil {
		return refuseAttach(call.Instance, "the links a previous start left could not be removed", err)
	}
	inode, err := namespaceInode(call.InitProcessID)
	if err != nil {
		return refuseAttach(call.Instance, "its network namespace could not be read", err)
	}
	for _, plan := range plans {
		input := &rpc.OwnedLinkCreateInput{
			Key:             call.Instance,
			Bridge:          plan.bridge,
			Slot:            plan.slot,
			PID:             call.InitProcessID,
			NamespaceInode:  inode,
			Name:            plan.name,
			HardwareAddress: plan.hardware,
			MTU:             plan.mtu,
			Addresses:       plan.addresses,
			Gateways:        plan.gateways,
		}
		if _, err := client.OwnedLinkCreate(ctx, input); err != nil {
			return refuseAttach(call.Instance, "its link on network "+plan.network+" could not be made",
				errors.Join(err, removeAfterFailure(ctx, client, call.Instance)))
		}
	}
	m.mu.Lock()
	m.live[call.Instance] = plans
	m.mu.Unlock()
	return nil
}

// removeAfterFailure removes every link recorded for instance after a failed
// created call, on a context of its own bounded by cleanupTimeout.
func removeAfterFailure(ctx context.Context, client container.EngineClient, instance string) error {
	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), cleanupTimeout)
	defer cancel()
	_, err := client.OwnedLinkRemove(cleanupCtx, &rpc.OwnedLinkRemoveInput{Key: instance})
	return err
}

// ended removes the instance's links. Only an instance whose created call
// made links has any: the guard on live is what keeps the start-up sweep,
// which runs before the engine answers, from sending a request. It releases
// no address: an address is released only when the config drops the
// attachment.
func (m *manager) ended(ctx context.Context, instance string) {
	m.mu.Lock()
	_, made := m.live[instance]
	delete(m.live, instance)
	client := m.client
	m.mu.Unlock()

	if !made {
		return
	}
	if client == nil {
		return
	}
	if _, err := client.OwnedLinkRemove(ctx, &rpc.OwnedLinkRemoveInput{Key: instance}); err != nil {
		// Going on is correct: the kernel removes a link pair with its
		// namespace, the interface component forgets a record whose device is
		// gone, and the instance's next start removes the record first.
		slogutil.Logger("container.network").Info("container links not removed at the end of the container; the next start removes them",
			"instance", instance, "error", err)
	}
}

func refuseAttach(instance, defect string, err error) error {
	return opererr.Wrap(err,
		"container "+instance+" was not started: "+defect,
		"a container attached to a network starts only once its links are in place",
		"request container restart name "+instance,
	)
}
