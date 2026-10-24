#!/usr/bin/perl
# Ze RADIUS interop lab: the relay that takes FreeRADIUS's own signature off.
#
# FreeRADIUS 3.2.10 cannot be configured to send an unsigned EAP answer.
# rad_encode (src/lib/radius.c in release_3_2_10) writes a Message-Authenticator
# as the FIRST attribute of every Access-Accept, Access-Reject and
# Access-Challenge it sends over UDP, whatever the reply list holds: the
# code2ma table decides it and no configuration item reaches that table. An
# unlang "update reply" therefore cannot remove it, and neither can a proxy
# server, which encodes through the same function.
#
# So this relay sits between ze and the server, inside the server's container.
# It forwards each Access-Request unchanged to the server, and each answer back
# to ze carrying every attribute the server wrote, in the server's order. In
# mode "strip" it removes the Message-Authenticator (type 80) first. In both
# modes it recomputes the Response Authenticator, RFC 2865 Section 3:
#
#   ResponseAuth = MD5(Code+ID+Length+RequestAuth+Attributes+Secret)
#
# The recomputation runs in "pass" mode too, where it reproduces the server's
# own value, so a relay that computed it wrongly fails the positive control
# before the stripped answer is ever read. The answer ze discards in "strip"
# mode then differs from one ze accepts by the missing attribute alone.
#
# The mode is read from a file on every request, so the checker switches it
# without restarting the relay. A missing or unknown mode forwards nothing and
# logs why: a relay that guessed a mode would make a run prove the wrong thing.
#
# One line per request goes to the log file, which the checker reads as the
# record of what the server answered and what the relay did to it.
#
# Usage: unsigned-relay.pl <listen-port> <server-host:port> <secret> <mode-file> <log-file>
use strict;
use warnings;
use Digest::MD5 qw(md5);
use IO::Select;
use IO::Socket::INET;

my ($listen_port, $server, $secret, $mode_file, $log_file) = @ARGV;
die "usage: unsigned-relay.pl <listen-port> <server-host:port> <secret> <mode-file> <log-file>\n"
	unless defined $log_file;

# RFC 2865 Section 3: the minimum and maximum length of a RADIUS packet.
my $header_octets = 20;
my $packet_octets_max = 4096;
# The time one request waits for the server's answer before it is logged as
# unanswered. The lab server answers a request it accepts within milliseconds,
# and ze's own retransmit timer is two seconds.
my $answer_wait_seconds = 3;

my $front = IO::Socket::INET->new(Proto => 'udp', LocalAddr => '0.0.0.0', LocalPort => $listen_port)
	or die "unsigned-relay: cannot listen on UDP port $listen_port: $!\n";
my $back = IO::Socket::INET->new(Proto => 'udp', PeerAddr => $server)
	or die "unsigned-relay: cannot reach the server at $server: $!\n";
open(my $log, '>>', $log_file) or die "unsigned-relay: cannot open $log_file: $!\n";
$log->autoflush(1);
print $log "listening port=$listen_port server=$server\n";

sub read_mode {
	open(my $fh, '<', $mode_file) or return 'missing';
	my $mode = <$fh>;
	close($fh);
	return 'empty' unless defined $mode;
	$mode =~ s/\s+//g;
	return $mode;
}

# rewrite walks the answer's attribute list and returns the list to send, or
# undef when the list does not walk. It records whether an EAP-Message (79)
# and a Message-Authenticator (80) were present.
sub rewrite {
	my ($attributes, $strip) = @_;
	my ($kept, $eap, $signature) = ('', 'absent', 'absent');
	my $offset = 0;
	while ($offset < length($attributes)) {
		return undef if $offset + 2 > length($attributes);
		my $type = ord(substr($attributes, $offset, 1));
		my $octets = ord(substr($attributes, $offset + 1, 1));
		return undef if $octets < 2;
		return undef if $offset + $octets > length($attributes);
		my $attribute = substr($attributes, $offset, $octets);
		$offset += $octets;
		$eap = 'present' if $type == 79;
		if ($type == 80) {
			$signature = $strip ? 'stripped' : 'present';
			next if $strip;
		}
		$kept .= $attribute;
	}
	return ($kept, $eap, $signature);
}

# The loop serves one request at a time for the life of the container. Ze's
# login sends one request and waits for its answer, so nothing queues behind it.
while (1) {
	my $client = $front->recv(my $request, $packet_octets_max);
	next unless defined $client;
	if (length($request) < $header_octets) {
		print $log "refused short-request octets=" . length($request) . "\n";
		next;
	}
	my $mode = read_mode();
	if ($mode ne 'pass' && $mode ne 'strip') {
		print $log "refused mode=$mode\n";
		next;
	}
	my $id = ord(substr($request, 1, 1));
	my $request_auth = substr($request, 4, 16);

	$back->send($request);
	my $answer;
	my $select = IO::Select->new($back);
	while ($select->can_read($answer_wait_seconds)) {
		$back->recv(my $candidate, $packet_octets_max);
		next if length($candidate) < $header_octets;
		next if ord(substr($candidate, 1, 1)) != $id;
		$answer = $candidate;
		last;
	}
	unless (defined $answer) {
		print $log "unanswered mode=$mode id=$id\n";
		next;
	}

	my $code = ord(substr($answer, 0, 1));
	my $octets = unpack('n', substr($answer, 2, 2));
	if ($octets < $header_octets || $octets > length($answer)) {
		print $log "refused malformed-answer mode=$mode id=$id\n";
		next;
	}
	my ($kept, $eap, $signature) = rewrite(substr($answer, $header_octets, $octets - $header_octets), $mode eq 'strip');
	unless (defined $kept) {
		print $log "refused malformed-attributes mode=$mode id=$id\n";
		next;
	}
	my $header = chr($code) . chr($id) . pack('n', $header_octets + length($kept));
	my $response_auth = md5($header . $request_auth . $kept . $secret);
	$front->send($header . $response_auth . $kept, 0, $client);
	print $log "relayed mode=$mode code=$code id=$id eap-message=$eap message-authenticator=$signature\n";
}
