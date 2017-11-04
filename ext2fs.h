#include "ext2fs/ext2_fs.h"
#include "ext2fs/ext2_types.h"
#include "ext2fs/ext2_err.h"
#include "ext2fs/ext2fs.h"
#include "ext2fs/ext2_io.h"
#include "et/com_err.h"
#include <stdlib.h>

errcode_t __ext2fs_open(char *name, int flags, int superblock, int block_size, io_manager manager, ext2_filsys *ret_fs);
errcode_t __WalkFunc_ext2fs_dir_iterate(ext2_filsys fs, ext2_ino_t dir, int flags, char *block_buf, void *private);
int* __allocCallback(int cbid);
void* __freeCallback(int* cbid);