package mp4f

import (
	"github.com/deepch/vdk/av"
	"github.com/deepch/vdk/format/fmp4/fmp4io"
	"github.com/deepch/vdk/format/mp4/mp4io"
	"github.com/deepch/vdk/format/mp4f/mp4fio"
	"time"
)

const GOPDuration = time.Millisecond * 500

// WritePacketV5 can return zero or one or two samples
func (m *Muxer) WritePacketV5(pkt av.Packet) [][]byte {
	var res [][]byte

	stream := m.streams[pkt.Idx]
	// if keyframe
	// or total size more than keyframe
	// or total duration more than GOPDuration - finish sample
	if pkt.IsKeyFrame ||
		len(stream.buffer)+len(pkt.Data) > m.keyframeSize ||
		stream.bufferDuration+pkt.Duration > GOPDuration {
		if stream.sampleIndex > 0 {
			stream.bufferDuration = 0
			res = append(res, stream.Finalize())
		}
	}

	stream.bufferDuration += pkt.Duration
	stream.WritePacketV5(pkt)

	if pkt.IsKeyFrame {
		stream.bufferDuration = 0
		m.keyframeSize = len(pkt.Data)
		res = append(res, stream.Finalize())
	}

	return res
}

func (s *Stream) WritePacketV5(pkt av.Packet) {
	defaultFlags := fmp4io.SampleNonKeyframe
	if pkt.IsKeyFrame {
		defaultFlags = fmp4io.SampleNoDependencies
	}
	trackID := pkt.Idx + 1
	if s.sampleIndex == 0 {
		s.moof.Header = &mp4fio.MovieFragHeader{Seqnum: uint32(s.muxer.fragmentIndex + 1)}
		s.moof.Tracks = []*mp4fio.TrackFrag{
			{
				Header: &mp4fio.TrackFragHeader{
					Data: []byte{0x00, 0x02, 0x00, 0x20, 0x00, 0x00, 0x00, uint8(trackID), 0x01, 0x01, 0x00, 0x00},
				},
				DecodeTime: &mp4fio.TrackFragDecodeTime{
					Version: 1,
					Flags:   0,
					Time:    uint64(s.dts),
				},
				Run: &mp4fio.TrackFragRun{
					Flags:            0x000b05,
					FirstSampleFlags: uint32(defaultFlags),
					DataOffset:       0,
					Entries:          []mp4io.TrackFragRunEntry{},
				},
			},
		}
		s.buffer = []byte{0x00, 0x00, 0x00, 0x00, 0x6d, 0x64, 0x61, 0x74}
	}
	runEntry := mp4io.TrackFragRunEntry{
		Duration: uint32(s.timeToTs(pkt.Duration)),
		Size:     uint32(len(pkt.Data)),
		Cts:      uint32(s.timeToTs(pkt.CompositionTime)),
		Flags:    uint32(defaultFlags),
	}
	s.moof.Tracks[0].Run.Entries = append(s.moof.Tracks[0].Run.Entries, runEntry)
	s.buffer = append(s.buffer, pkt.Data...)
	s.sampleIndex++
	s.dts += s.timeToTs(pkt.Duration)
}

func (s *Stream) Finalize() []byte {
	s.moof.Tracks[0].Run.DataOffset = uint32(s.moof.Len() + 8)
	out := make([]byte, s.moof.Len()+len(s.buffer))
	s.moof.Marshal(out)
	PutU32BE(s.buffer, uint32(len(s.buffer)))
	copy(out[s.moof.Len():], s.buffer)
	s.sampleIndex = 0
	s.muxer.fragmentIndex++
	return out
}
