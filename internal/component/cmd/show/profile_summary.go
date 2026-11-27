// Design: docs/architecture/diagnostics/procfs-diagnostics.md -- pure-stdlib pprof protobuf summary decoder
// Related: profile.go -- show system profile handler and | summary pipe registration

package show

import (
	"bytes"
	"cmp"
	"compress/gzip"
	"encoding/binary"
	"errors"
	"io"
	"math"
	"slices"
	"strings"
)

const (
	// maxProfileDecompressBytes bounds gzip decompression to protect the daemon
	// from oversized or crafted profile payloads.
	maxProfileDecompressBytes = 32 << 20

	// maxProfileSummaryEntries caps the top-functions slice returned by
	// show system profile | summary.
	maxProfileSummaryEntries = 32

	protoWireVarint  = 0
	protoWireFixed64 = 1
	protoWireBytes   = 2
	protoWireFixed32 = 5

	pprofFieldSampleType        = 1
	pprofFieldSample            = 2
	pprofFieldLocation          = 4
	pprofFieldFunction          = 5
	pprofFieldStringTable       = 6
	pprofFieldDefaultSampleType = 14

	pprofValueTypeFieldType = 1
	pprofValueTypeFieldUnit = 2

	pprofSampleFieldLocation = 1
	pprofSampleFieldValue    = 2

	pprofLocationFieldID   = 1
	pprofLocationFieldLine = 4

	pprofLineFieldFunctionID = 1
	pprofLineFieldLine       = 2

	pprofFunctionFieldID       = 1
	pprofFunctionFieldName     = 2
	pprofFunctionFieldFilename = 4
)

var (
	errProfileEmpty              = errors.New("profile payload is empty")
	errProfileTooLarge           = errors.New("profile exceeds 32 MiB decompressed limit")
	errProfileCorruptWire        = errors.New("profile contains malformed protobuf wire data")
	errProfileInvalidStringIndex = errors.New("profile contains out-of-range string table index")
	errProfileNoSampleTypes      = errors.New("profile contains no sample types")
	errProfileSampleValueCount   = errors.New("profile sample value count does not match sample types")
)

type pprofValueType struct {
	typeIdx int64
	unitIdx int64
}

type pprofResolvedSampleType struct {
	typeName string
	unitName string
}

type pprofSample struct {
	locationIDs []uint64
	values      []int64
}

type pprofLine struct {
	functionID uint64
	line       int64
}

type pprofLocation struct {
	id    uint64
	lines []pprofLine
}

type pprofFunction struct {
	id          uint64
	nameIdx     int64
	filenameIdx int64
}

type pprofResolvedFunction struct {
	name     string
	filename string
}

type pprofProfile struct {
	sampleTypes          []pprofValueType
	samples              []pprofSample
	locations            []pprofLocation
	functions            []pprofFunction
	stringTable          []string
	defaultSampleType    int64
	hasDefaultSampleType bool
}

type pprofFuncKey struct {
	function string
	file     string
	line     int64
}

type pprofFuncAggregate struct {
	key  pprofFuncKey
	flat int64
	cum  int64
}

// summarizePprof decodes a gzipped perftools.profiles.Profile protobuf snapshot
// and aggregates flat and cumulative totals per function.
func summarizePprof(profileType string, raw []byte) (map[string]any, error) {
	decompressed, err := gunzipProfile(raw)
	if err != nil {
		return nil, err
	}
	prof, err := decodePprof(decompressed)
	if err != nil {
		return nil, err
	}
	if len(prof.sampleTypes) == 0 {
		return nil, errProfileNoSampleTypes
	}

	sampleTypes, err := resolvePprofSampleTypes(prof.sampleTypes, prof.stringTable)
	if err != nil {
		return nil, err
	}

	defaultType := ""
	if prof.hasDefaultSampleType {
		defaultType, err = pprofString(prof.stringTable, prof.defaultSampleType)
		if err != nil {
			return nil, err
		}
	}

	funcByID, err := resolvePprofFunctions(prof.functions, prof.stringTable)
	if err != nil {
		return nil, err
	}

	locByID := make(map[uint64][]pprofLine, len(prof.locations))
	for i := range prof.locations {
		locByID[prof.locations[i].id] = prof.locations[i].lines
	}

	valIdx := selectProfileValueIndex(profileType, sampleTypes, defaultType)
	selected := sampleTypes[valIdx]

	total, aggregates, err := aggregatePprofSamples(prof.samples, len(sampleTypes), valIdx, locByID, funcByID)
	if err != nil {
		return nil, err
	}

	slices.SortFunc(aggregates, comparePprofAggregates)

	totalFuncs := len(aggregates)
	limit := min(totalFuncs, maxProfileSummaryEntries)
	rows := make([]map[string]any, 0, limit)
	for i := range limit {
		agg := aggregates[i]
		rows = append(rows, map[string]any{
			"function": agg.key.function,
			"file":     agg.key.file,
			"line":     agg.key.line,
			"flat":     agg.flat,
			"flat-pct": profilePercent(agg.flat, total),
			"cum":      agg.cum,
			"cum-pct":  profilePercent(agg.cum, total),
		})
	}

	return map[string]any{
		keyType:             profileType,
		keyFormat:           pipeOpSummary,
		"sample-type":       selected.typeName,
		"sample-unit":       selected.unitName,
		triageKeyTotal:      total,
		"top-functions":     rows,
		keyCount:            len(rows),
		triageKeyTotalCount: totalFuncs,
		triageKeyTruncated:  totalFuncs > limit,
	}, nil
}

func gunzipProfile(raw []byte) ([]byte, error) {
	if len(raw) == 0 {
		return nil, errProfileEmpty
	}
	gz, err := gzip.NewReader(bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	decompressed, readErr := io.ReadAll(io.LimitReader(gz, maxProfileDecompressBytes+1))
	closeErr := gz.Close()
	if readErr != nil {
		return nil, readErr
	}
	if len(decompressed) > maxProfileDecompressBytes {
		return nil, errProfileTooLarge
	}
	if closeErr != nil {
		return nil, closeErr
	}
	if len(decompressed) == 0 {
		return nil, errProfileEmpty
	}
	return decompressed, nil
}

func decodePprof(data []byte) (pprofProfile, error) {
	var prof pprofProfile
	for len(data) > 0 {
		fieldNum, wireType, rest, err := readProtoTag(data)
		if err != nil {
			return pprofProfile{}, err
		}
		switch fieldNum {
		case pprofFieldSampleType:
			sub, next, err := readProtoDelimited(rest, wireType)
			if err != nil {
				return pprofProfile{}, err
			}
			vt, err := decodePprofValueType(sub)
			if err != nil {
				return pprofProfile{}, err
			}
			prof.sampleTypes = append(prof.sampleTypes, vt)
			data = next
		case pprofFieldSample:
			sub, next, err := readProtoDelimited(rest, wireType)
			if err != nil {
				return pprofProfile{}, err
			}
			s, err := decodePprofSample(sub)
			if err != nil {
				return pprofProfile{}, err
			}
			prof.samples = append(prof.samples, s)
			data = next
		case pprofFieldLocation:
			sub, next, err := readProtoDelimited(rest, wireType)
			if err != nil {
				return pprofProfile{}, err
			}
			loc, err := decodePprofLocation(sub)
			if err != nil {
				return pprofProfile{}, err
			}
			prof.locations = append(prof.locations, loc)
			data = next
		case pprofFieldFunction:
			sub, next, err := readProtoDelimited(rest, wireType)
			if err != nil {
				return pprofProfile{}, err
			}
			fn, err := decodePprofFunction(sub)
			if err != nil {
				return pprofProfile{}, err
			}
			prof.functions = append(prof.functions, fn)
			data = next
		case pprofFieldStringTable:
			sub, next, err := readProtoDelimited(rest, wireType)
			if err != nil {
				return pprofProfile{}, err
			}
			prof.stringTable = append(prof.stringTable, string(sub))
			data = next
		case pprofFieldDefaultSampleType:
			val, next, err := readProtoVarintField(rest, wireType)
			if err != nil {
				return pprofProfile{}, err
			}
			prof.defaultSampleType = int64(val)
			prof.hasDefaultSampleType = true
			data = next
		default:
			next, err := skipProtoField(rest, wireType)
			if err != nil {
				return pprofProfile{}, err
			}
			data = next
		}
	}
	return prof, nil
}

func decodePprofValueType(data []byte) (pprofValueType, error) {
	var vt pprofValueType
	for len(data) > 0 {
		fieldNum, wireType, rest, err := readProtoTag(data)
		if err != nil {
			return pprofValueType{}, err
		}
		switch fieldNum {
		case pprofValueTypeFieldType:
			val, next, err := readProtoVarintField(rest, wireType)
			if err != nil {
				return pprofValueType{}, err
			}
			vt.typeIdx = int64(val)
			data = next
		case pprofValueTypeFieldUnit:
			val, next, err := readProtoVarintField(rest, wireType)
			if err != nil {
				return pprofValueType{}, err
			}
			vt.unitIdx = int64(val)
			data = next
		default:
			next, err := skipProtoField(rest, wireType)
			if err != nil {
				return pprofValueType{}, err
			}
			data = next
		}
	}
	return vt, nil
}

func decodePprofSample(data []byte) (pprofSample, error) {
	var s pprofSample
	for len(data) > 0 {
		fieldNum, wireType, rest, err := readProtoTag(data)
		if err != nil {
			return pprofSample{}, err
		}
		switch fieldNum {
		case pprofSampleFieldLocation:
			ids, next, err := readRepeatedUint64(rest, wireType, s.locationIDs)
			if err != nil {
				return pprofSample{}, err
			}
			s.locationIDs = ids
			data = next
		case pprofSampleFieldValue:
			vals, next, err := readRepeatedInt64(rest, wireType, s.values)
			if err != nil {
				return pprofSample{}, err
			}
			s.values = vals
			data = next
		default:
			next, err := skipProtoField(rest, wireType)
			if err != nil {
				return pprofSample{}, err
			}
			data = next
		}
	}
	return s, nil
}

func decodePprofLocation(data []byte) (pprofLocation, error) {
	var loc pprofLocation
	for len(data) > 0 {
		fieldNum, wireType, rest, err := readProtoTag(data)
		if err != nil {
			return pprofLocation{}, err
		}
		switch fieldNum {
		case pprofLocationFieldID:
			val, next, err := readProtoVarintField(rest, wireType)
			if err != nil {
				return pprofLocation{}, err
			}
			loc.id = val
			data = next
		case pprofLocationFieldLine:
			sub, next, err := readProtoDelimited(rest, wireType)
			if err != nil {
				return pprofLocation{}, err
			}
			ln, err := decodePprofLine(sub)
			if err != nil {
				return pprofLocation{}, err
			}
			loc.lines = append(loc.lines, ln)
			data = next
		default:
			next, err := skipProtoField(rest, wireType)
			if err != nil {
				return pprofLocation{}, err
			}
			data = next
		}
	}
	return loc, nil
}

func decodePprofLine(data []byte) (pprofLine, error) {
	var ln pprofLine
	for len(data) > 0 {
		fieldNum, wireType, rest, err := readProtoTag(data)
		if err != nil {
			return pprofLine{}, err
		}
		switch fieldNum {
		case pprofLineFieldFunctionID:
			val, next, err := readProtoVarintField(rest, wireType)
			if err != nil {
				return pprofLine{}, err
			}
			ln.functionID = val
			data = next
		case pprofLineFieldLine:
			val, next, err := readProtoVarintField(rest, wireType)
			if err != nil {
				return pprofLine{}, err
			}
			ln.line = int64(val)
			data = next
		default:
			next, err := skipProtoField(rest, wireType)
			if err != nil {
				return pprofLine{}, err
			}
			data = next
		}
	}
	return ln, nil
}

func decodePprofFunction(data []byte) (pprofFunction, error) {
	var fn pprofFunction
	for len(data) > 0 {
		fieldNum, wireType, rest, err := readProtoTag(data)
		if err != nil {
			return pprofFunction{}, err
		}
		switch fieldNum {
		case pprofFunctionFieldID:
			val, next, err := readProtoVarintField(rest, wireType)
			if err != nil {
				return pprofFunction{}, err
			}
			fn.id = val
			data = next
		case pprofFunctionFieldName:
			val, next, err := readProtoVarintField(rest, wireType)
			if err != nil {
				return pprofFunction{}, err
			}
			fn.nameIdx = int64(val)
			data = next
		case pprofFunctionFieldFilename:
			val, next, err := readProtoVarintField(rest, wireType)
			if err != nil {
				return pprofFunction{}, err
			}
			fn.filenameIdx = int64(val)
			data = next
		default:
			next, err := skipProtoField(rest, wireType)
			if err != nil {
				return pprofFunction{}, err
			}
			data = next
		}
	}
	return fn, nil
}

func readProtoTag(data []byte) (fieldNum, wireType uint64, rest []byte, err error) {
	tag, n := binary.Uvarint(data)
	if n <= 0 {
		return 0, 0, nil, errProfileCorruptWire
	}
	fieldNum = tag >> 3
	wireType = tag & 7
	if fieldNum == 0 {
		return 0, 0, nil, errProfileCorruptWire
	}
	return fieldNum, wireType, data[n:], nil
}

func readProtoVarintField(data []byte, wireType uint64) (uint64, []byte, error) {
	if wireType != protoWireVarint {
		return 0, nil, errProfileCorruptWire
	}
	val, n := binary.Uvarint(data)
	if n <= 0 {
		return 0, nil, errProfileCorruptWire
	}
	return val, data[n:], nil
}

func readProtoDelimited(data []byte, wireType uint64) (sub, rest []byte, err error) {
	if wireType != protoWireBytes {
		return nil, nil, errProfileCorruptWire
	}
	size, n := binary.Uvarint(data)
	if n <= 0 {
		return nil, nil, errProfileCorruptWire
	}
	body := data[n:]
	if size > uint64(len(body)) {
		return nil, nil, errProfileCorruptWire
	}
	return body[:size], body[size:], nil
}

func readRepeatedUint64(data []byte, wireType uint64, dst []uint64) ([]uint64, []byte, error) {
	switch wireType {
	case protoWireVarint:
		val, n := binary.Uvarint(data)
		if n <= 0 {
			return nil, nil, errProfileCorruptWire
		}
		return append(dst, val), data[n:], nil
	case protoWireBytes:
		packed, rest, err := readProtoDelimited(data, wireType)
		if err != nil {
			return nil, nil, err
		}
		for len(packed) > 0 {
			val, n := binary.Uvarint(packed)
			if n <= 0 {
				return nil, nil, errProfileCorruptWire
			}
			dst = append(dst, val)
			packed = packed[n:]
		}
		return dst, rest, nil
	default:
		return nil, nil, errProfileCorruptWire
	}
}

func readRepeatedInt64(data []byte, wireType uint64, dst []int64) ([]int64, []byte, error) {
	switch wireType {
	case protoWireVarint:
		val, n := binary.Uvarint(data)
		if n <= 0 {
			return nil, nil, errProfileCorruptWire
		}
		return append(dst, int64(val)), data[n:], nil
	case protoWireBytes:
		packed, rest, err := readProtoDelimited(data, wireType)
		if err != nil {
			return nil, nil, err
		}
		for len(packed) > 0 {
			val, n := binary.Uvarint(packed)
			if n <= 0 {
				return nil, nil, errProfileCorruptWire
			}
			dst = append(dst, int64(val))
			packed = packed[n:]
		}
		return dst, rest, nil
	default:
		return nil, nil, errProfileCorruptWire
	}
}

func skipProtoField(data []byte, wireType uint64) ([]byte, error) {
	switch wireType {
	case protoWireVarint:
		_, n := binary.Uvarint(data)
		if n <= 0 {
			return nil, errProfileCorruptWire
		}
		return data[n:], nil
	case protoWireFixed64:
		if len(data) < 8 {
			return nil, errProfileCorruptWire
		}
		return data[8:], nil
	case protoWireBytes:
		_, rest, err := readProtoDelimited(data, wireType)
		return rest, err
	case protoWireFixed32:
		if len(data) < 4 {
			return nil, errProfileCorruptWire
		}
		return data[4:], nil
	default:
		return nil, errProfileCorruptWire
	}
}

func pprofString(table []string, idx int64) (string, error) {
	if idx < 0 {
		return "", errProfileInvalidStringIndex
	}
	if uint64(idx) >= uint64(len(table)) {
		return "", errProfileInvalidStringIndex
	}
	return table[idx], nil
}

func resolvePprofSampleTypes(raw []pprofValueType, table []string) ([]pprofResolvedSampleType, error) {
	resolved := make([]pprofResolvedSampleType, 0, len(raw))
	for i := range raw {
		typeName, err := pprofString(table, raw[i].typeIdx)
		if err != nil {
			return nil, err
		}
		unitName, err := pprofString(table, raw[i].unitIdx)
		if err != nil {
			return nil, err
		}
		resolved = append(resolved, pprofResolvedSampleType{
			typeName: typeName,
			unitName: unitName,
		})
	}
	return resolved, nil
}

func resolvePprofFunctions(raw []pprofFunction, table []string) (map[uint64]pprofResolvedFunction, error) {
	resolved := make(map[uint64]pprofResolvedFunction, len(raw))
	for i := range raw {
		name, err := pprofString(table, raw[i].nameIdx)
		if err != nil {
			return nil, err
		}
		filename, err := pprofString(table, raw[i].filenameIdx)
		if err != nil {
			return nil, err
		}
		resolved[raw[i].id] = pprofResolvedFunction{
			name:     name,
			filename: filename,
		}
	}
	return resolved, nil
}

func selectProfileValueIndex(profileType string, sampleTypes []pprofResolvedSampleType, defaultType string) int {
	preferred := preferredSampleTypeNames(profileType)
	for _, want := range preferred {
		if idx := findSampleTypeIndex(sampleTypes, want); idx >= 0 {
			return idx
		}
	}
	if defaultType != "" {
		if idx := findSampleTypeIndex(sampleTypes, defaultType); idx >= 0 {
			return idx
		}
	}
	if profileType == profileTypeGoroutine {
		return 0
	}
	return len(sampleTypes) - 1
}

func preferredSampleTypeNames(profileType string) []string {
	switch profileType {
	case profileTypeHeap:
		return []string{"inuse_space"}
	case profileTypeAllocs:
		return []string{"alloc_space"}
	case profileTypeCPU:
		return []string{profileTypeCPU, "samples"}
	case profileTypeGoroutine:
		return []string{profileTypeGoroutine}
	case profileTypeMutex, profileTypeBlock:
		return []string{"delay", "contentions"}
	default:
		return nil
	}
}

func findSampleTypeIndex(sampleTypes []pprofResolvedSampleType, name string) int {
	for i := range sampleTypes {
		if sampleTypes[i].typeName == name {
			return i
		}
	}
	return -1
}

func aggregatePprofSamples(
	samples []pprofSample,
	sampleTypeCount, valIdx int,
	locByID map[uint64][]pprofLine,
	funcByID map[uint64]pprofResolvedFunction,
) (int64, []pprofFuncAggregate, error) {
	totals := make(map[pprofFuncKey]*pprofFuncAggregate)
	var grandTotal int64

	for i := range samples {
		s := &samples[i]
		if len(s.values) != sampleTypeCount {
			return 0, nil, errProfileSampleValueCount
		}
		val := s.values[valIdx]
		if val <= 0 {
			continue
		}
		grandTotal += val
		if len(s.locationIDs) == 0 {
			continue
		}

		// Attribute flat value to the innermost line of frame 0 (leaf frame).
		if leafLines := locByID[s.locationIDs[0]]; len(leafLines) > 0 {
			if fn, ok := funcByID[leafLines[0].functionID]; ok {
				key := pprofFuncKey{function: fn.name, file: fn.filename, line: leafLines[0].line}
				entry := getOrCreateAggregate(totals, key)
				entry.flat += val
			}
		}

		// Attribute cumulative value once per unique function key in this sample.
		seen := make(map[pprofFuncKey]struct{}, len(s.locationIDs))
		for _, locID := range s.locationIDs {
			for _, ln := range locByID[locID] {
				fn, ok := funcByID[ln.functionID]
				if !ok {
					continue
				}
				key := pprofFuncKey{function: fn.name, file: fn.filename, line: ln.line}
				if _, dup := seen[key]; dup {
					continue
				}
				seen[key] = struct{}{}
				entry := getOrCreateAggregate(totals, key)
				entry.cum += val
			}
		}
	}

	out := make([]pprofFuncAggregate, 0, len(totals))
	for _, entry := range totals {
		out = append(out, *entry)
	}
	return grandTotal, out, nil
}

func getOrCreateAggregate(totals map[pprofFuncKey]*pprofFuncAggregate, key pprofFuncKey) *pprofFuncAggregate {
	entry, ok := totals[key]
	if !ok {
		entry = &pprofFuncAggregate{key: key}
		totals[key] = entry
	}
	return entry
}

func comparePprofAggregates(a, b pprofFuncAggregate) int {
	if c := cmp.Compare(b.cum, a.cum); c != 0 {
		return c
	}
	if c := cmp.Compare(b.flat, a.flat); c != 0 {
		return c
	}
	if c := strings.Compare(a.key.function, b.key.function); c != 0 {
		return c
	}
	if c := strings.Compare(a.key.file, b.key.file); c != 0 {
		return c
	}
	return cmp.Compare(a.key.line, b.key.line)
}

func profilePercent(part, total int64) float64 {
	if total <= 0 {
		return 0
	}
	return math.Round(float64(part)*10000/float64(total)) / 100
}
