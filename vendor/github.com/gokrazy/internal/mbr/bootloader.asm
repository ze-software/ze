; Minimal Linux Bootloader
; ========================

; @ author:	Sebastian Plotz
; @ version:	1.0
; @ date:	24.07.2012

; Copyright (C) 2012 Sebastian Plotz

; Minimal Linux Bootloader is free software: you can redistribute it and/or modify
; it under the terms of the GNU General Public License as published by
; the Free Software Foundation, either version 3 of the License, or
; (at your option) any later version.

; Minimal Linux Bootloader is distributed in the hope that it will be useful,
; but WITHOUT ANY WARRANTY; without even the implied warranty of
; MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
; GNU General Public License for more details.

; You should have received a copy of the GNU General Public License
; along with Minimal Linux Bootloader. If not, see <http://www.gnu.org/licenses/>.

; Memory layout
; =============

; 0x07c00 - 0x07dff	Mininal Linux Bootloader
;			+ partition table
;			+ MBR signature
; 0x10000 - 0x17fff	Real mode kernel
; 0x18000 - 0x1dfff	Stack and heap
; 0x1e000 - 0x1ffff	Kernel command line
; 0x20000 - 0x2fdff	temporal space for
;			protected-mode kernel

; base_ptr = 0x10000
; heap_end = 0xe000
; heap_end_ptr = heap_end - 0x200 = 0xde00
; cmd_line_ptr = base_ptr + heap_end = 0x1e000

; [bits 16] is not necessary when using `nasm bootloader.asm`
; but it becomes necessary when switching to `nasm -f elf32 bootloader.asm`
[bits 16]

; org 0x7c00 is necessary when using `nasm bootloader.asm`
; but it stops working when switching to `nasm -f elf32 bootloader.asm`
;org	0x7c00

	cli
	xor	ax, ax
	mov	ds, ax
	mov	ss, ax
	mov	sp, 0x7c00			; setup stack ...
	mov	ax, 0x1000
	mov	es, ax
	sti

read_cmdline:

	mov	eax, 0x0001			; load one sector
	xor	bx, bx				; no offset
	mov	cx, 0x1e00			; load Kernel command line at 0x1e000
	mov	esi, cmd_lba
	call	read_from_hdd

read_kernel_bootsector:

	mov	eax, 0x0001			; load one sector
	xor	bx, bx				; no offset
	mov	cx, 0x1000			; load Kernel boot sector at 0x10000
	mov	esi, current_lba
	call	read_from_hdd

read_kernel_setup:

	xor	eax, eax
	mov	al, [es:0x1f1]			; no. of sectors to load
	cmp	ax, 0				; 4 if setup_sects = 0
	jne	read_kernel_setup.next
	mov	ax, 4
.next:
	mov	bx, 512				; 512 byte offset
	mov	cx, 0x1000
	mov	esi, current_lba
	call	read_from_hdd

check_version:

	cmp	word [es:0x206], 0x204		; we need protocol version >= 2.04
	jb	error
	test	byte [es:0x211], 1
	jz	error

set_header_fields:

	mov	byte [es:0x210], 0xe1		; set type_of_loader
	or	byte [es:0x211], 0x80		; set CAN_USE_HEAP
	mov	word [es:0x224], 0xde00		; set heap_end_ptr
	;mov	byte [es:0x226], 0x00		; set ext_loader_ver
	mov	byte [es:0x227], 0x01		; set ext_loader_type (bootloader id: 0x11)
	mov	dword [es:0x228], 0x1e000	; set cmd_line_ptr
	cld					; copy cmd_line

read_protected_mode_kernel:

	mov	edx, [es:0x1f4]			; edx stores the number of bytes to load
	shl	edx, 4
.loop:
	cmp	edx, 0
	je	run_kernel
	cmp	edx, 0xfe00			; less than 127*512 bytes remaining?
	jb	read_protected_mode_kernel_2
	mov	eax, 0x7f			; load 127 sectors (maximum)
	xor	bx, bx				; no offset
	mov	cx, 0x2000			; load temporary to 0x20000
	mov	esi, current_lba
	call	read_from_hdd
	mov	cx, 0x7f00			; move 65024 bytes (127*512 byte)
	call	do_move
	sub	edx, 0xfe00			; update the number of bytes to load
	add	word [gdt.dest], 0xfe00
	adc	byte [gdt.dest+2], 0
	adc	byte [gdt.dest+5], 0
	jmp	short read_protected_mode_kernel.loop

read_protected_mode_kernel_2:

	mov	eax, edx
	shr	eax, 9
	test	edx, 511
	jz	read_protected_mode_kernel_2.next
	inc	eax
.next:
	xor	bx, bx
	mov	cx, 0x2000
	mov	esi, current_lba
	call	read_from_hdd
	mov	ecx, edx
	shr	ecx, 1
	call	do_move

run_kernel:

	cli
	mov	ax, 0x1000
	mov	ds, ax
	mov	es, ax
	mov	fs, ax
	mov	gs, ax
	mov	ss, ax
	mov	sp, 0xe000
	jmp	0x1020:0

	;; read_from_hdd:
	;;   ax: count in 512-byte sectors [1, 127]
	;;   bx: destination: offset
	;;   cx: destination: segment
	;;   esi: lba pointer (typically current_lba)
read_from_hdd:

	push	edx
	mov	[dap.count], ax
	mov	[dap.offset], bx
	mov	[dap.segment], cx
	mov	edx, [esi]
	mov	[dap.lba], edx
	add	[esi], eax			; update current_lba
	mov	ah, 0x42
	mov	si, dap
	mov	dl, 0x80			; first hard disk
	int	0x13
	jc	error
	pop	edx
	ret

do_move:

	push	edx
	push	es
	xor	ax, ax
	mov	es, ax
	mov	ah, 0x87
	mov	si, gdt
	int	0x15
	jc	error
	pop	es
	pop	edx
	ret

error:

	mov	si, error_msg

; This used to be a loop, but we needed space for fixing
; https://github.com/gokrazy/gokrazy/issues/248, so now
; only the first letter (E) is printed on error.
msg_loop:

	lodsb
	mov	ah, 0xe
	mov	bx, 7
	int	0x10
	;; fall-through to reboot
	;;
	;; (The padding is here just to reach 440 bytes.)
	nop

reboot:

	xor	ax, ax
	int	0x16
	int	0x19
	jmp	0xf000:0xfff0			; BIOS reset code

; Global Descriptor Table

gdt:

	times	16	db	0
	dw	0xffff				; segment limit
.src:
	dw	0
	db	2
	db	0x93				; data access rights
	dw	0
	dw	0xffff				; segment limit
.dest:
	dw	0
	db	0x10				; load protected-mode kernel to 100000h
	db	0x93				; data access rights
	dw	0
	times	16	db	0

; Disk Address Packet

dap:

	db	0x10				; size of DAP
	db	0				; unused
.count:
	dw	0				; number of sectors
.offset:
	dw	0				; destination: offset
.segment:
	dw	0				; destination: segment
.lba:
	dd	0				; low bytes of LBA address
	dd	0				; high bytes of LBA address

error_msg	db	'er', 0		; /* FIXME: newline */
current_lba	dd	8218			; initialize to first LBA address
cmd_lba	dd	8218			; initialize to LBA address of cmdline.txt
