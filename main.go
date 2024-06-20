// Copyright 2019-2024 The NATS Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"encoding/binary"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

const (
	// hdrLen
	hdrLen = 2
	// This is where we keep the message store blocks.
	msgDir = "msgs"

	// Metafiles for streams and consumers.
	JetStreamMetaFile    = "meta.inf"
	JetStreamMetaFileSum = "meta.sum"
	JetStreamMetaFileKey = "meta.key"

	// This is the full snapshotted state for the stream.
	streamStreamStateFile = "index.db"

	// Default stream block size.
	defaultLargeBlockSize = 8 * 1024 * 1024 // 8MB
	// max block size for now.
	maxBlockSize = defaultLargeBlockSize
	// FileStoreMinBlkSize is minimum size we will do for a blk size.
	FileStoreMinBlkSize = 32 * 1000 // 32kib
	// FileStoreMaxBlkSize is maximum size we will do for a blk size.
	FileStoreMaxBlkSize = maxBlockSize
)

// StreamState is information about the given stream.
type StreamState struct {
	Msgs        uint64            `json:"messages"`
	Bytes       uint64            `json:"bytes"`
	FirstSeq    uint64            `json:"first_seq"`
	FirstTime   time.Time         `json:"first_ts"`
	LastSeq     uint64            `json:"last_seq"`
	LastTime    time.Time         `json:"last_ts"`
	NumSubjects int               `json:"num_subjects,omitempty"`
	Subjects    map[string]uint64 `json:"subjects,omitempty"`
	NumDeleted  int               `json:"num_deleted,omitempty"`
	Deleted     []uint64          `json:"deleted,omitempty"`
	Lost        *LostStreamData   `json:"lost,omitempty"`
	Consumers   int               `json:"consumer_count"`
}

// SimpleState for filtered subject specific state.
type SimpleState struct {
	Msgs  uint64 `json:"messages"`
	First uint64 `json:"first_seq"`
	Last  uint64 `json:"last_seq"`
}

// LostStreamData indicates msgs that have been lost.
type LostStreamData struct {
	Msgs  []uint64 `json:"msgs"`
	Bytes uint64   `json:"bytes"`
}

// SnapshotResult contains information about the snapshot.
type SnapshotResult struct {
	Reader io.ReadCloser
	State  StreamState
}

func main() {
	dir := flag.String("dir", "", "")
	flag.Parse()
	fn := filepath.Join(*dir, msgDir, streamStreamStateFile)
	buf, err := os.ReadFile(fn)
	if err != nil {
		panic(err)
	}
	bi := hdrLen

	readU64 := func() uint64 {
		if bi < 0 {
			return 0
		}
		v, n := binary.Uvarint(buf[bi:])
		if n <= 0 {
			bi = -1
			return 0
		}
		bi += n
		return v
	}
	readI64 := func() int64 {
		if bi < 0 {
			return 0
		}
		v, n := binary.Varint(buf[bi:])
		if n <= 0 {
			bi = -1
			return -1
		}
		bi += n
		return v
	}

	setTime := func(t *time.Time, ts int64) {
		if ts == 0 {
			*t = time.Time{}
		} else {
			*t = time.Unix(0, ts).UTC()
		}
	}

	var state StreamState
	state.Msgs = readU64()
	state.Bytes = readU64()
	state.FirstSeq = readU64()
	baseTime := readI64()
	setTime(&state.FirstTime, baseTime)
	state.LastSeq = readU64()
	setTime(&state.LastTime, readI64())

	s, _ := json.Marshal(state)
	fmt.Println(string(s))

	if numSubjects := int(readU64()); numSubjects > 0 {
		for i := 0; i < numSubjects; i++ {
			if lsubj := int(readU64()); lsubj > 0 {
				subj := buf[bi : bi+lsubj]
				fmt.Println(string(subj))
				bi += lsubj
			}
		}
	}

	if numBlocks := readU64(); numBlocks > 0 {
		lastIndex := int(numBlocks - 1)
		fmt.Println(lastIndex)
		for i := 0; i < int(numBlocks); i++ {
			index, nbytes, fseq, fts, lseq, lts, numDeleted := uint32(readU64()), readU64(), readU64(), readI64(), readU64(), readI64(), readU64()
			if bi < 0 {
				break
			}
			fmt.Printf("index: %d, nbytes: %d, fseq: %d, fts: %d, lseq: %d, lts: %d, numDeleted: %d\n", index, nbytes, fseq, fts, lseq, lts, numDeleted)
		}
	}
}
