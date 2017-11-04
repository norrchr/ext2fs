// +build !windows

package ext2fs

/*
#cgo linux LDFLAGS: -lext2fs
#include "./ext2fs.h"

extern int WalkFunc(struct ext2_dir_entry* p0, int p1, int p2, char* p3, void* p4);
*/
import "C"
import (
	"errors"
	"fmt"
	"sync/atomic"
	"unsafe"
	"sort"
)

type ext2fs struct {
	handle     C.ext2_filsys
	ioManager  C.io_manager
	open       bool
	path       string
}

type BlockType int

const (
	EXT2_DATA_BLOCK = iota
	EXT2_EXTENT_BLOCK
	EXT2_DIR_BLOCK
)

type Block uint64
type Blocks struct {
	Start Block
	End   Block
}

//FilesystemObject A FilesystemObject (basically an Inode)
type FilesystemObject struct {
	Inode  uint64
	Type   BlockType
	Blocks []Blocks
	Path   string
}

//Stats Stats structure
type Stats struct {
	Dirs  uint64
	Files uint64
}

type walk struct {
	handle       C.ext2_filsys
	dir          C.ext2_ino_t
	parent       string
	blocks       map[uint64]struct{}
	inodes       map[uint64]struct{}
	objects      []FilesystemObject
	stats        Stats
}

type rawBlocks []uint64
func (b rawBlocks) Len() int           { return len(b) }
func (b rawBlocks) Swap(i, j int)      { b[i], b[j] = b[j], b[i] }
func (b rawBlocks) Less(i, j int) bool { return b[i] < b[j] }

var (
	cbid      int64
	private = make(map[int64]interface{})
)

//New Return new initialised ext2fs object
func New() *ext2fs {
	return &ext2fs{}
}

//IsOpen returns true if open, else false
func (fs *ext2fs) IsOpen() bool {
	return fs.open
}

//Open Initialise a filesystem handle
func (fs *ext2fs) Open(path string, flags, superBlock, blockSize int) error {

	if fs.IsOpen() {
		return errors.New("handle already open")
	}

	fs.ioManager = C.unix_io_manager

	cpath := C.CString(path)
	defer C.free(unsafe.Pointer(cpath))

	var ret C.errcode_t
	ret, _ = C.__ext2fs_open(cpath, C.int(flags), C.int(superBlock), C.int(blockSize), fs.ioManager, (*C.ext2_filsys)(&fs.handle))

	if ret > 0 {
		return fmt.Errorf("ext2fs_open error: %d", ret)
	}

	fs.path = path
	fs.open = true
	return nil
}

//Close Close filesystem handle

func (fs *ext2fs) Close() error {

	if !fs.IsOpen() {
		return errors.New("Filesystem handle not open")
	}

	var ret C.errcode_t
	ret, _ = C.ext2fs_close(fs.handle)

	if ret > 0 {
		return fmt.Errorf("ext2fs_close error: %d", ret)
	}

	fs.open = false
	return nil
}

//LookupFilesystemObjects Given a list of inodes and/or blocks, return a list of FilesystemObjects
func (fs *ext2fs) LookupFilesystemObjects(inodes []uint64, blocks []uint64) ([]FilesystemObject, Stats, error) {
	var scan C.ext2_inode_scan
	var ino C.ext2_ino_t
	var inode C.struct_ext2_inode
	var ret C.errcode_t
	var blockBuf *C.char
	var parent *C.char
	var p0 uint64

	prv := &walk{}

	if !fs.IsOpen() {
		return prv.objects, prv.stats, errors.New("Filesystem handle not open")
	}

	_cbid := C.__allocCallback(C.int(atomic.AddInt64(&cbid, 1)))
	if *_cbid <= C.int(0) {
		return prv.objects, prv.stats, errors.New("Could not alloc callback")
	}
	defer C.__freeCallback(_cbid)

	private[cbid] = prv

	prv.handle = fs.handle

	prv.blocks = make(map[uint64]struct{})
	prv.inodes = make(map[uint64]struct{})

	for _, block := range blocks {
		prv.blocks[block] = struct{}{}
	}
	for _, i := range inodes {
		prv.inodes[i] = struct{}{}
	}

	ret, _ = C.ext2fs_open_inode_scan(fs.handle, C.int(0), &scan)
	if ret > 0 {
		return prv.objects, prv.stats, fmt.Errorf("ext2fs_open_inode_scan error: %d", ret)
	}

	for {
		ret, _ = C.ext2fs_get_next_inode(scan, &ino, &inode)
		if ret == C.EXT2_ET_BAD_BLOCK_IN_INODE_TABLE {
			continue
		}

		if ret > 0 || ino == 0 {
			break
		}

		if ino > 0 {

			if inode.i_links_count <= 0 {
				continue
			}

			if C.ext2fs_check_directory(fs.handle, ino) > 0 {
				continue
			}

			prv.dir = ino
			prv.parent = ""
			prv.stats.Dirs++

			ret, _ = C.ext2fs_get_pathname(fs.handle, prv.dir, C.ext2_ino_t(0), &parent)
			if ret > 0 {
				prv.parent = ""
			} else {
				prv.parent = C.GoString(parent)
			}
			C.ext2fs_free_mem(unsafe.Pointer(&parent))

			for p0 = range prv.inodes {
				if (C.ext2_ino_t)(p0) == ino {
					prv.objects = append(prv.objects, FilesystemObject{
						Inode: p0,
						Type: EXT2_DIR_BLOCK,
						Path: prv.parent,
					})
					delete(prv.inodes, p0)
					break
				}
			}

			ret, _ = C.__WalkFunc_ext2fs_dir_iterate(fs.handle, ino, C.int(0), blockBuf, unsafe.Pointer(_cbid))
			if ret > 0 {
				if ret == C.DIRENT_ABORT {
					break
				}
				continue
			}

			if len(prv.inodes) + len(prv.blocks) <= 0 {
				break
			}

		}

	}

	return prv.objects, prv.stats, nil
}

//export WalkFunc
func WalkFunc(dirent *C.struct_ext2_dir_entry, offset C.int, blocksize C.int, buf *C.char, cbid unsafe.Pointer) C.int {
	var _cbid = (*C.int)(unsafe.Pointer(cbid))
	var ino C.ext2_ino_t = (C.ext2_ino_t)(dirent.inode)
	var inode C.struct_ext2_inode
	var blocks []Blocks
	var Type BlockType
	var p0 uint64
	var name string
	var ret C.errcode_t

	prv, ok := private[int64(*_cbid)].(*walk)
	if !ok {
		return C.int(-1)
	}

	if C.ext2fs_check_directory(prv.handle, ino) <= 0 {
		return C.int(0)
	}

	name = C.GoStringN(&dirent.name[0], C.int(dirent.name_len & 0xFF))
	if name == "." || name == ".." {
		return C.int(0)
	}

	ret, _ = C.ext2fs_read_inode(prv.handle, ino, &inode)

	if inode.i_flags & C.EXT4_EXTENTS_FL != 0 {
		var handle C.ext2_extent_handle_t
		var extent, next C.struct_ext2fs_extent
		var op C.int = C.EXT2_EXTENT_ROOT
		var blkcount C.e2_blkcnt_t = (C.e2_blkcnt_t)(0)
		var start, end C.blk64_t

		Type = EXT2_EXTENT_BLOCK

		ret, _ = C.ext2fs_extent_open2(prv.handle, ino, &inode, &handle)
		if ret > 0 {
			return C.int(0)
		}

		for {
			if op == C.EXT2_EXTENT_CURRENT {
				ret = 0
			} else {
				ret = C.ext2fs_extent_get(handle, op, &extent)
			}
			if ret > 0 {
				if ret != C.EXT2_ET_EXTENT_NO_NEXT {
					break
				}
				ret = 0
				//next_block_set??
				break
			}

			op = C.EXT2_EXTENT_NEXT

			if extent.e_flags & C.EXT2_EXTENT_FLAGS_LEAF == 0 {
				//we only care about data blocks
				continue
			}

			ret, _ = C.ext2fs_extent_get(handle, op, &next)

			if extent.e_lblk + (C.blk64_t)(extent.e_len) <= (C.blk64_t)(blkcount) {
				continue
			}
			blkcount = (C.e2_blkcnt_t)(extent.e_lblk)

			start = extent.e_pblk
			end = start + (C.blk64_t)(extent.e_len) - C.blk64_t(1)

			blocks = append(blocks, Blocks{Block(start), Block(end)})

			if ret == 0 {
				extent = next
				op = C.EXT2_EXTENT_CURRENT
			}
		}

		C.ext2fs_extent_free(handle)

	} else { 

		Type = EXT2_DATA_BLOCK

		var rblocks rawBlocks
		for i := C.__u32(0); i < C.EXT2_NDIR_BLOCKS; i++ {
			rblocks = append(rblocks, uint64(inode.i_block[i]))
		}
		sort.Sort(rblocks)

		var start, end = rblocks[0], rblocks[0]
		var i, j int
		
		for i = 0; i <= len(rblocks); j++  {
			if i+j >= len(rblocks) {
				j--
				start, end = rblocks[i], rblocks[i+j]
				blocks = append(blocks, Blocks{Block(start), Block(end)})
				break			
			}
			if rblocks[i] + uint64(j) == rblocks[i+j] {
				end = rblocks[i+j]
			} else {
				start, end = rblocks[i+j], rblocks[i+j]
				blocks = append(blocks, Blocks{Block(start), Block(end)})
				i += j
				j = 0
			}
		}
	
	}

	prv.stats.Files++

	for p0 = range prv.inodes {
		if (C.ext2_ino_t)(p0) == ino {
			prv.objects = append(prv.objects, FilesystemObject{
				Inode: p0,
				Type: Type,
				Blocks: blocks,
				Path: fmt.Sprintf("%s/%s", prv.parent, name),
			})
			delete(prv.inodes, p0)
			break
		}
	}

	var blk Block
	for p0 = range prv.blocks {
		blk = Block(p0)
		for _, block := range blocks {
			if blk >= block.Start && blk <= block.End {
				prv.objects = append(prv.objects, FilesystemObject{
					Inode: uint64(ino),
					Type: Type,
					Blocks: blocks,
					Path: fmt.Sprintf("%s/%s", prv.parent, name),
				})
				delete(prv.blocks, p0)
			}
		}
	}

	if len(prv.inodes) + len(prv.blocks) <= 0 {
		return C.DIRENT_ABORT
	}

	return C.int(0)
}