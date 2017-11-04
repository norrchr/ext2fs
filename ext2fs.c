#include "./ext2fs.h"
#include "_cgo_export.h"

errcode_t __ext2fs_open(char *name, int flags, int superblock, int block_size, io_manager manager, ext2_filsys *ret_fs)
{
 return ext2fs_open((const char *)name, flags, superblock, block_size, manager, ret_fs);
}

errcode_t __WalkFunc_ext2fs_dir_iterate(ext2_filsys fs, ext2_ino_t dir, int flags, char *block_buf, void *private) {
	return ext2fs_dir_iterate(fs, dir, flags, block_buf, WalkFunc, private);
}

int* __allocCallback(int cbid) {
	int *p = (int *)malloc(sizeof(int));
	if (p <= 0) {
		return;
	}
	*p = cbid;
	return p;
}

void* __freeCallback(int* cbid) {
	free(cbid);
}