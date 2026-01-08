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
    print "watchdog withdraw\n";
    sleep 5;

    # specify a watchdog name (which may be the same or different each time)
    print "watchdog withdraw watchdog-one\n";
    sleep 5;

    print "watchdog announce\n";
    sleep 5;

    print "watchdog announce watchdog-one\n";
    sleep 5;

    # In our example, there are no routes that are tied to these watchdogs, but we may after a future config reload
    print "watchdog announce watchdog-two\n";
    print "watchdog withdraw watchdog-two\n";
}
