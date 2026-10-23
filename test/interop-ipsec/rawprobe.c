//go:build ignore

/*
 * The constraint above keeps this file out of the Go package beside it
 * (parity_test.go): Ze builds with CGO_ENABLED=0, and Go refuses a package
 * holding a C file it would have to compile, so every build of the package
 * failed. gcc reads the line as a comment; Dockerfile.strongswan builds it.
 *
 * rawprobe: send exactly one crafted IPv4 UDP datagram or TCP SYN through a raw
 * IP socket, so the packet takes the kernel's routing and XFRM output path and is
 * protected by whatever IPsec policy covers it.
 *
 * The IPsec lab's strongSwan image carries it for craftedProbe and injectOutOfSA
 * (internal/le/interoplab/ipsec). It replaces nping, whose 0.7.99 release (the
 * Alpine 3.24 nmap-nping 7.99) ignores --send-ip and always writes the frame at the
 * Ethernet layer. Measured 2026-10-22: nping --send-ip reported "Raw packets sent:
 * 1 (42B)", a 14-byte Ethernet header on a 28-byte datagram, and an iptables OUTPUT
 * rule matching the datagram counted 0 packets where nc counted 1. A frame written
 * there bypasses XFRM, leaves in the clear, and every probe assertion over the
 * Child SA reads the peer's policy drop instead of its checksum verdict.
 *
 * The kernel writes the IPv4 header (no IP_HDRINCL); this program writes the UDP or
 * TCP header and its checksum, which a raw socket never computes. With -b the
 * checksum is deliberately wrong, which is what a NAT leaves behind on a
 * transport-mode flow (RFC 3948 Section 3.1.2).
 *
 * usage: rawprobe -p udp|tcp -d DPORT [-s SPORT] [-x PAYLOAD-HEX] [-b] TARGET
 */
#include <arpa/inet.h>
#include <errno.h>
#include <netinet/in.h>
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/socket.h>
#include <unistd.h>

#define MAX_SEGMENT 1400

static void usage(void)
{
	fprintf(stderr, "usage: rawprobe -p udp|tcp -d DPORT [-s SPORT] [-x PAYLOAD-HEX] [-b] TARGET\n");
	exit(2);
}

static int parse_port(const char *text, const char *what)
{
	char *end = NULL;
	long value = strtol(text, &end, 10);
	if (end == text || *end != '\0' || value < 1 || value > 65535) {
		fprintf(stderr, "rawprobe: %s %s is not a port in 1..65535\n", what, text);
		exit(2);
	}
	return (int)value;
}

/* The payload is hex digits, with or without a leading 0x (the form the lab's
 * payload constants carry). */
static size_t parse_hex(const char *text, uint8_t *out, size_t room)
{
	if (text[0] == '0' && (text[1] == 'x' || text[1] == 'X'))
		text += 2;
	size_t length = strlen(text);
	if (strspn(text, "0123456789abcdefABCDEF") != length) {
		fprintf(stderr, "rawprobe: payload hex %s holds a non-hex digit\n", text);
		exit(2);
	}
	if (length % 2 != 0 || length / 2 > room) {
		fprintf(stderr, "rawprobe: payload hex must hold an even number of digits and at most %zu bytes\n", room);
		exit(2);
	}
	for (size_t i = 0; i < length / 2; i++) {
		unsigned int byte;
		if (sscanf(text + 2 * i, "%2x", &byte) != 1) {
			fprintf(stderr, "rawprobe: payload hex %s holds a non-hex digit\n", text);
			exit(2);
		}
		out[i] = (uint8_t)byte;
	}
	return length / 2;
}

static uint32_t sum16(const uint8_t *data, size_t length, uint32_t sum)
{
	for (size_t i = 0; i + 1 < length; i += 2)
		sum += (uint32_t)data[i] << 8 | data[i + 1];
	if (length % 2 != 0)
		sum += (uint32_t)data[length - 1] << 8;
	return sum;
}

/* RFC 768 and RFC 9293 Section 3.1: the checksum covers a pseudo header of the
 * source and destination addresses, the protocol and the segment length. */
static uint16_t transport_checksum(struct in_addr source, struct in_addr target, int protocol,
				   const uint8_t *segment, size_t length)
{
	uint8_t pseudo[12];
	memcpy(pseudo, &source.s_addr, 4);
	memcpy(pseudo + 4, &target.s_addr, 4);
	pseudo[8] = 0;
	pseudo[9] = (uint8_t)protocol;
	pseudo[10] = (uint8_t)(length >> 8);
	pseudo[11] = (uint8_t)length;
	uint32_t sum = sum16(segment, length, sum16(pseudo, sizeof pseudo, 0));
	while (sum >> 16)
		sum = (sum & 0xffff) + (sum >> 16);
	uint16_t result = (uint16_t)~sum;
	/* RFC 768: a computed UDP checksum of zero is transmitted as all ones. */
	if (protocol == IPPROTO_UDP && result == 0)
		result = 0xffff;
	return result;
}

/* The source address the kernel will write, learned from a connected UDP socket,
 * because the checksum's pseudo header must carry it. */
static struct in_addr route_source(struct in_addr target)
{
	struct sockaddr_in remote = {.sin_family = AF_INET, .sin_port = htons(9), .sin_addr = target};
	struct sockaddr_in local;
	socklen_t size = sizeof local;
	int fd = socket(AF_INET, SOCK_DGRAM, 0);
	if (fd < 0 || connect(fd, (struct sockaddr *)&remote, sizeof remote) != 0 ||
	    getsockname(fd, (struct sockaddr *)&local, &size) != 0) {
		fprintf(stderr, "rawprobe: no route to %s: %s\n", inet_ntoa(target), strerror(errno));
		exit(1);
	}
	close(fd);
	return local.sin_addr;
}

int main(int argc, char **argv)
{
	const char *protocol_name = NULL;
	int dport = 0, sport = 0, bad = 0, option;
	uint8_t payload[MAX_SEGMENT];
	size_t payload_length = 0;

	while ((option = getopt(argc, argv, "p:d:s:x:b")) != -1) {
		switch (option) {
		case 'p': protocol_name = optarg; break;
		case 'd': dport = parse_port(optarg, "destination port"); break;
		case 's': sport = parse_port(optarg, "source port"); break;
		case 'x': payload_length = parse_hex(optarg, payload, sizeof payload); break;
		case 'b': bad = 1; break;
		default: usage();
		}
	}
	if (protocol_name == NULL || dport == 0 || optind != argc - 1)
		usage();

	int protocol;
	size_t header;
	if (strcmp(protocol_name, "udp") == 0) {
		protocol = IPPROTO_UDP;
		header = 8;
		if (sport == 0)
			sport = 53;
	} else if (strcmp(protocol_name, "tcp") == 0) {
		protocol = IPPROTO_TCP;
		header = 20;
		if (sport == 0)
			sport = 40000;
	} else {
		fprintf(stderr, "rawprobe: protocol %s is neither udp nor tcp\n", protocol_name);
		exit(2);
	}

	struct in_addr target;
	if (inet_pton(AF_INET, argv[optind], &target) != 1) {
		fprintf(stderr, "rawprobe: target %s is not an IPv4 address\n", argv[optind]);
		exit(2);
	}
	struct in_addr source = route_source(target);

	uint8_t segment[MAX_SEGMENT + 20];
	size_t length = header + payload_length;
	memset(segment, 0, header);
	segment[0] = (uint8_t)(sport >> 8);
	segment[1] = (uint8_t)sport;
	segment[2] = (uint8_t)(dport >> 8);
	segment[3] = (uint8_t)dport;
	size_t checksum_at;
	if (protocol == IPPROTO_UDP) {
		/* RFC 768: source port, destination port, length, checksum. */
		segment[4] = (uint8_t)(length >> 8);
		segment[5] = (uint8_t)length;
		checksum_at = 6;
	} else {
		/* RFC 9293 Section 3.1: sequence number, data offset 5, SYN, window. */
		segment[4] = 0x5a;
		segment[5] = 0x5e;
		segment[6] = 0x01;
		segment[7] = 0x02;
		segment[12] = 5 << 4;
		segment[13] = 0x02;
		segment[14] = 0x04;
		checksum_at = 16;
	}
	memcpy(segment + header, payload, payload_length);

	uint16_t checksum = transport_checksum(source, target, protocol, segment, length);
	if (bad) {
		/* Any other non-zero value: zero would mean "no checksum" to a UDP receiver. */
		checksum ^= 0x5a5a;
		if (checksum == 0)
			checksum = 0x1234;
	}
	segment[checksum_at] = (uint8_t)(checksum >> 8);
	segment[checksum_at + 1] = (uint8_t)checksum;

	int fd = socket(AF_INET, SOCK_RAW, protocol);
	if (fd < 0) {
		fprintf(stderr, "rawprobe: raw socket: %s (needs CAP_NET_RAW)\n", strerror(errno));
		return 1;
	}
	struct sockaddr_in remote = {.sin_family = AF_INET, .sin_addr = target};
	if (sendto(fd, segment, length, 0, (struct sockaddr *)&remote, sizeof remote) != (ssize_t)length) {
		fprintf(stderr, "rawprobe: send to %s: %s\n", argv[optind], strerror(errno));
		return 1;
	}
	close(fd);
	printf("SENT %s %s:%d > %s:%d len=%zu csum=0x%04x%s\n", protocol_name, inet_ntoa(source), sport,
	       argv[optind], dport, length, checksum, bad ? " (corrupt)" : "");
	return 0;
}
