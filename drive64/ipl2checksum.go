package drive64

import (
	"encoding/binary"
)

const MAGIC_NUMBER = 0x6c078965

type ReadCallback func(uint32) uint32

type CheckSumInfo struct {
	Input        []byte
	Buffer       [16]uint32
	ChecksumLow  uint32
	ChecksumHigh uint32
}

func checksumFunction(a0, a1, a2 uint32) uint32 {
	var prod uint64
	var hi, lo, diff, res uint32

	if a1 == 0 {
		a1 = a2
	}

	prod = uint64(a0) * uint64(a1)
	hi = uint32(prod >> 32)
	lo = uint32(prod)
	diff = hi - lo
	if diff == 0 {
		res = a0
	} else {
		res = diff
	}
	return res
}

func initializeChecksum(seed uint8, input []byte) *CheckSumInfo {
	var init, data uint32
	var info CheckSumInfo

	info.Input = input

	init = MAGIC_NUMBER*uint32(seed) + 1
	data = binary.BigEndian.Uint32(input[0:4])
	init ^= data

	for loop := 0; loop < 16; loop++ {
		info.Buffer[loop] = init
	}

	return &info
}

func calculateChecksum(info *CheckSumInfo) {
	var sum, dataIndex, loop, shift, dataShiftedRight, dataShiftedLeft, s2Tmp, s5Tmp, a3Tmp uint32
	var data, dataNext, dataLast uint32

	if info == nil {
		return
	}

	dataIndex = 0
	loop = 0
	data = binary.BigEndian.Uint32(info.Input[0:4])
	for {
		loop++
		dataLast = data
		data = binary.BigEndian.Uint32(info.Input[dataIndex : dataIndex+4])

		sum = checksumFunction(1007-loop, data, loop)
		info.Buffer[0] += sum

		sum = checksumFunction(info.Buffer[1], data, loop)
		info.Buffer[1] = sum
		info.Buffer[2] ^= data

		sum = checksumFunction(data+5, MAGIC_NUMBER, loop)
		info.Buffer[3] += sum

		if dataLast < data {
			sum = checksumFunction(info.Buffer[9], data, loop)
			info.Buffer[9] = sum
		} else {
			info.Buffer[9] += data
		}

		shift = dataLast & 0x1f
		dataShiftedRight = data >> shift
		dataShiftedLeft = data << (32 - shift)
		s5Tmp = dataShiftedRight | dataShiftedLeft
		info.Buffer[4] += s5Tmp

		dataShiftedLeft = data << shift
		dataShiftedRight = data >> (32 - shift)

		sum = checksumFunction(info.Buffer[7], dataShiftedLeft|dataShiftedRight, loop)
		info.Buffer[7] = sum

		if data < info.Buffer[6] {
			info.Buffer[6] = (info.Buffer[3] + info.Buffer[6]) ^ (data + loop)
		} else {
			info.Buffer[6] = (info.Buffer[4] + data) ^ info.Buffer[6]
		}

		shift = dataLast >> 27
		dataShiftedRight = data >> (32 - shift)
		dataShiftedLeft = data << shift
		s2Tmp = dataShiftedLeft | dataShiftedRight
		info.Buffer[5] += s2Tmp

		dataShiftedLeft = data << (32 - shift)
		dataShiftedRight = data >> shift

		sum = checksumFunction(info.Buffer[8], dataShiftedRight|dataShiftedLeft, loop)
		info.Buffer[8] = sum

		if loop == 1008 {
			break
		}

		dataIndex += 4
		dataNext = binary.BigEndian.Uint32(info.Input[dataIndex : dataIndex+4])

		sum = checksumFunction(info.Buffer[15], s2Tmp, loop)

		shift = data >> 27
		dataShiftedLeft = dataNext << shift
		dataShiftedRight = dataNext >> (32 - shift)

		sum = checksumFunction(sum, dataShiftedLeft|dataShiftedRight, loop)
		info.Buffer[15] = sum

		sum = checksumFunction(info.Buffer[14], s5Tmp, loop)

		shift = data & 0x1f
		s2Tmp = shift
		dataShiftedLeft = dataNext << (32 - shift)
		dataShiftedRight = dataNext >> shift

		sum = checksumFunction(sum, dataShiftedRight|dataShiftedLeft, loop)
		info.Buffer[14] = sum

		dataShiftedRight = data >> s2Tmp
		dataShiftedLeft = data << (32 - s2Tmp)
		a3Tmp = dataShiftedRight | dataShiftedLeft

		shift = dataNext & 0x1f
		dataShiftedRight = dataNext >> shift
		dataShiftedLeft = dataNext << (32 - shift)

		info.Buffer[13] += a3Tmp + (dataShiftedRight | dataShiftedLeft)

		sum = checksumFunction(info.Buffer[10]+data, dataNext, loop)
		info.Buffer[10] = sum

		sum = checksumFunction(info.Buffer[11]^data, dataNext, loop)
		info.Buffer[11] = sum

		info.Buffer[12] += info.Buffer[8] ^ data
	}
}

func finalizeChecksum(info *CheckSumInfo) {
	var buf [4]uint32
	var checksum uint64
	var sum, loop, tmp, s2Tmp, shift, dataShiftedRight, dataShiftedLeft uint32

	if info == nil {
		return
	}

	data := info.Buffer[0]
	buf[0] = data
	buf[1] = data
	buf[2] = data
	buf[3] = data

	for loop = 0; loop < 16; loop++ {
		data = info.Buffer[loop]

		shift = data & 0x1f
		dataShiftedLeft = data << (32 - shift)
		dataShiftedRight = data >> shift
		tmp = buf[0] + (dataShiftedRight | dataShiftedLeft)
		buf[0] = tmp

		if data < tmp {
			buf[1] += data
		} else {
			sum = checksumFunction(buf[1], data, loop)
			buf[1] = sum
		}

		tmp = (data & 0x02) >> 1
		s2Tmp = data & 0x01

		if tmp == s2Tmp {
			buf[2] += data
		} else {
			sum = checksumFunction(buf[2], data, loop)
			buf[2] = sum
		}

		if s2Tmp == 1 {
			buf[3] ^= data
		} else {
			sum = checksumFunction(buf[3], data, loop)
			buf[3] = sum
		}
	}

	// 0xa4001510
	sum = checksumFunction(buf[0], buf[1], 16)
	tmp = buf[3] ^ buf[2]

	checksum = uint64(sum) << 32
	checksum |= uint64(tmp)
	checksum &= 0xffffffffffff

	info.ChecksumLow = uint32(checksum)
	info.ChecksumHigh = uint32(checksum >> 32)
}

func IPL2Checksum(input []byte, seed uint8) uint64 {
	info := initializeChecksum(seed, input)
	calculateChecksum(info)
	finalizeChecksum(info)
	return uint64(info.ChecksumHigh)<<32 | uint64(info.ChecksumLow)
}
