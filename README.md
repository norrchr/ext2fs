# ext2fs

[https://godoc.org/github.com/r0t3n/ext2fs][godoc]

A golang alternative to using **debugfs** to lookup filesystem objects via inode and block number(s).

Makes use of the **libext2fs** C library from [e2fsprogs] based on 1.42.11 api

# Example usage

dd if=/dev/urandom of=~/out bs=1M count=1 conv=fdatasync
1+0 records in
1+0 records out
1048576 bytes (1.0 MB) copied, 0.173941 s, 6.0 MB/s

ls -i ~/out
19420 /root/out

./example -s -device /dev/sda1 -inode 19420
Processed 1842 dirs, 13079 files, 1 matches
Inode   Filename        [flags] #Blocks Blocks
19420   /root/out      [e]#255 1445632-1445887

# Block types

**EXT2_DIR_BLOCK**
: Is a directory (no blocks returned)

**EXT2_EXTENT_BLOCK**
: Uses extent block layout

**EXT2_DATA_BLOCK**
: Uses data blocks layout

[godoc]: <https://godoc.org/github.com/r0t3n/ext2fs>
[e2fsprogs]: <https://github.com/tytso/e2fsprogs>