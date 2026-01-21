#!/bin/sh

# ignore Control C
# if the user ^C exabgp we will get that signal too, ignore it and let exabgp send us a SIGTERM
trap '' SIGINT

# command and watchdog name are case sensitive

while `true`;
do

# Let give exabgp the time to setup the BGP session :)
# But we do not have too, exabgp will record the changes and update the routes once up otherwise

sleep 10

# without name exabgp will use the name of the service as watchdog name
echo "watchdog withdraw"
sleep 5

# specify a watchdog name (which may be the same or different each time)
echo "watchdog withdraw watchdog-one"
sleep 5

echo "watchdog announce"
sleep 5

echo "watchdog announce watchdog-one"
sleep 5

# we have no route with that watchdog but it does not matter, we could have after a configuration reload

echo "watchdog announce watchdog-two"
echo "watchdog withdraw watchdog-two"

done
