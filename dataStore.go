package blockexplorer

import (
	"sort"
	"sync"
)

var l sync.Mutex
var m map[uint32]Data
var Size []int
var SaveTime []int64
var Spand []uint64

func init() {
	m = map[uint32]Data{}
}

type Data struct {
	SaveTime int64
	Spand    uint64
}

func Save(key uint32, v int64, v2 uint64) {
	l.Lock()
	defer l.Unlock()
	m[key] = Data{
		SaveTime: v,
		Spand:    v2,
	}
	if v > 0 {
		SaveTime = insertSortInt64(SaveTime, v)
	}
	if v2 > 0 {
		Spand = insertSortUint64(Spand, v2)
	}
}
func SaveSize(i int) {
	Size = insertSortInt(Size, i)
}
func Get(key uint32) *Data {
	l.Lock()
	defer l.Unlock()
	if d, has := m[key]; has {
		return &d
	}
	return nil
}
func GetSaveTime(persent int) int64 {
	l.Lock()
	defer l.Unlock()
	index := int((float64(len(SaveTime)) / 100) * float64(persent))
	if index >= len(SaveTime) {
		return 0
	}
	return SaveTime[index]
}
func GetSpand(persent int) uint64 {
	l.Lock()
	defer l.Unlock()
	index := int((float64(len(Spand)) / 100) * float64(persent))
	if index >= len(Spand) {
		return 0
	}
	return Spand[index]
}
func GetSize(persent int) int {
	l.Lock()
	defer l.Unlock()
	index := int((float64(len(Size)) / 100) * float64(persent))
	if index >= len(Size) {
		return 0
	}
	return Size[index]
}

func insertSortInt(data []int, el int) []int {
	index := sort.Search(len(data), func(i int) bool { return data[i] > el })
	data = append(data, -1)
	copy(data[index+1:], data[index:])
	data[index] = el
	return data
}
func insertSortInt64(data []int64, el int64) []int64 {
	index := sort.Search(len(data), func(i int) bool { return data[i] > el })
	data = append(data, -1)
	copy(data[index+1:], data[index:])
	data[index] = el
	return data
}
func insertSortUint64(data []uint64, el uint64) []uint64 {
	index := sort.Search(len(data), func(i int) bool { return data[i] > el })
	data = append(data, 0)
	copy(data[index+1:], data[index:])
	data[index] = el
	return data
}
