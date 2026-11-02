package game

import "github.com/openorion2/openorion2/internal/random"

// RUSS.C helpers shared by the turn: Fisher–Yates shuffles and weighted
// picks. Each shuffle draws Random(n-i) for i = 0..n-1, even when i is kept.

// shuffleChar is Shuffle_Char_ (0xfe9b5).
func shuffleChar(r *random.Source, a []uint8) {
	n := int16(len(a))
	for i := int16(0); i < n; i++ {
		j := int16(int32(r.Random(uint32(int32(n)-int32(i)))) + int32(i) - 1)
		a[i], a[j] = a[j], a[i]
	}
}

// shuffleSint is Shuffle_Sint_ (0xfe9f5).
func shuffleSint(r *random.Source, a []int16) {
	n := int16(len(a))
	for i := int16(0); i < n; i++ {
		j := int16(int32(r.Random(uint32(int32(n)-int32(i)))) + int32(i) - 1)
		a[i], a[j] = a[j], a[i]
	}
}

// shuffleInt is Shuffle_Int_ (0xfea3c); Shuffle_ (0xfea82) sends 4-byte
// items here.
func shuffleInt(r *random.Source, a []uint32) {
	n := int16(len(a))
	for i := int16(0); i < n; i++ {
		j := int16(int32(r.Random(uint32(int32(n)-int32(i)))) + int32(i) - 1)
		a[i], a[j] = a[j], a[i]
	}
}

// getWeightedChoiceInt is Get_Weighted_Choice_Int_ (0xfe96f): -1 without a
// draw when the weights sum to 0.
func getWeightedChoiceInt(r *random.Source, w []uint32) int16 {
	var sum uint32
	for _, v := range w {
		sum += v
	}
	if sum == 0 {
		return -1
	}
	left := r.Random(sum)
	i := int16(0)
	for left > w[i] {
		left -= w[i]
		i++
	}
	return i
}

// getWeightedChoiceShort is Get_Weighted_Choice_Short_ (0xfe92d).
func getWeightedChoiceShort(r *random.Source, w []uint16) int16 {
	var sum uint32
	for _, v := range w {
		sum += uint32(v)
	}
	if sum == 0 {
		return -1
	}
	left := r.Random(sum)
	i := int16(0)
	for uint32(w[i]) < left {
		left -= uint32(w[i])
		i++
	}
	return i
}

// getWeightedChoiceChar is Get_Weighted_Choice_Char_ (0xfe8da); the walk
// compares only the low 16 bits of the draw.
func getWeightedChoiceChar(r *random.Source, w []uint8) int16 {
	var sum uint32
	for _, v := range w {
		sum += uint32(v)
	}
	if sum == 0 {
		return -1
	}
	left := uint16(r.Random(sum))
	i := int16(0)
	for int32(w[i]) < int32(left) {
		left -= uint16(w[i])
		i++
	}
	return i
}
