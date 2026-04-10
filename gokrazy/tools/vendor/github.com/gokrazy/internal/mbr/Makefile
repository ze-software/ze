# see also https://copyprogramming.com/howto/how-to-make-an-os-kernel
.PHONY: all

all:
	nasm -f elf32 -g3 -F dwarf bootloader.asm -o bootloader.o
	ld -Ttext=0x7c00 -melf_i386 bootloader.o -o bootloader.elf
	objcopy -O binary bootloader.elf bootloader.img
