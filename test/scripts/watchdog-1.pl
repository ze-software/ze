#!/usr/bin/perl

use strict;
use warnings;

# Ignore Control C
$SIG{'INT'} = sub {};

# make STDOUT unbuffered
select STDOUT; $| = 1;

while (1) {

    # Give exabgp time to establish the session :)
    sleep 10;

    # without a name, exabgp will use the name of the service as the name of the watchdog
    # Step 5: watchdog commands moved to bgp namespace
    print "bgp watchdog withdraw\n";
    sleep 5;

    # specify a watchdog name (which may be the same or different each time)
    print "bgp watchdog withdraw watchdog-one\n";
    sleep 5;

    print "bgp watchdog announce\n";
    sleep 5;

    print "bgp watchdog announce watchdog-one\n";
    sleep 5;

    # In our example, there are no routes that are tied to these watchdogs, but we may after a future config reload
    print "bgp watchdog announce watchdog-two\n";
    print "bgp watchdog withdraw watchdog-two\n";
}
