---
title: Terminal Demonstrations
description: Recorded Ze command, configuration, monitoring, and access-control workflows from reproducible tape files.
category: observe
journey: Evaluate
---
# Terminal Demonstrations

These recordings run real Ze commands against isolated local fixtures. Each checked-in tape file defines every keystroke, pause, and terminal size. A release can therefore regenerate the recordings when Ze changes.

A terminal demo is an asciicast. The page replays it as text, so you can select a command and copy it. The one browser demo stays a video. The recordings use no public service, and the player is served from this site.

Each demo also appears beside the documentation for the feature it exercises. The transcript below each player provides the same command sequence without replaying the recording.

## Interactive command launcher

<!-- terminal-demo: launcher -->

## Live BGP dashboard

<!-- terminal-demo: cli-dashboard -->

## ZeFS and SSH configuration

<!-- terminal-demo: zefs-config -->

## Read-only operator access

<!-- terminal-demo: rbac -->

## Traceroute in an isolated Linux lab

<!-- terminal-demo: traceroute -->

## Web configuration commit

<!-- terminal-demo: web-config -->

## Confirmed commit rollback

<!-- terminal-demo: commit-confirmed -->

## RPKI validation enforcement

<!-- terminal-demo: rpki -->

## Route installation from BGP RIB to Linux FIB

<!-- terminal-demo: rib-fib -->

## Live warnings and retained errors

<!-- The health-reports demo is not embedded while its recording is wrong. Its
     tape drives an SSH session whose commands the CLI answers with a completion
     listing rather than a result, so the recording holds the intro card, a
     config box and the recap, and none of the output this gallery describes.
     Restore this marker when the recording shows the session again.
     Recorded in plan/journal/green-that-could-not-have-been-red.md -->
<!-- terminal-demo-disabled: health-reports -->

## Configuration views and formatter pipes

<!-- terminal-demo: config-views -->

## BFD-triggered BGP failover

<!-- terminal-demo: bfd-failover -->

## OSPF adjacency and learned route

<!-- terminal-demo: ospf-adjacency -->

## Live traffic attribution

<!-- terminal-demo: traffic-anomaly -->

## VRRP gateway failover

<!-- terminal-demo: vrrp-failover -->

## Offline Linux host inventory

<!-- terminal-demo: host-inventory -->

## Configuration dependency impact

<!-- terminal-demo: config-graph -->
