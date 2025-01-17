package manager

import (
	"fmt"
	"hash/fnv"
	"sort"
	"strconv"
	"time"

	"github.com/influxdata/telegraf"
)

type metric struct {
	name   string
	tags   []*telegraf.Tag
	fields []*telegraf.Field
	tm     time.Time

	tp        telegraf.ValueType
	aggregate bool
}

func NewMetric(
	name string,
	tags map[string]string,
	fields map[string]interface{},
	tm time.Time,
	tp ...telegraf.ValueType,
) (telegraf.Metric, error) {
	var vtype telegraf.ValueType
	if len(tp) > 0 {
		vtype = tp[0]
	} else {
		vtype = telegraf.Untyped
	}

	m := &metric{
		name:   name,
		tags:   nil,
		fields: nil,
		tm:     tm,
		tp:     vtype,
	}

	if len(tags) > 0 {
		m.tags = make([]*telegraf.Tag, 0, len(tags))
		for k, v := range tags {
			m.tags = append(m.tags,
				&telegraf.Tag{Key: k, Value: v})
		}
		sort.Slice(m.tags, func(i, j int) bool { return m.tags[i].Key < m.tags[j].Key })
	}

	if len(fields) > 0 {
		m.fields = make([]*telegraf.Field, 0, len(fields))
		for k, v := range fields {
			v := convertField(v)
			if v == nil {
				continue
			}
			m.AddField(k, v)
		}
	}

	return m, nil
}

// FromMetric returns a deep copy of the metric with any tracking information
// removed.
func FromMetric(other telegraf.Metric) telegraf.Metric {
	m := &metric{
		name:      other.Name(),
		tags:      make([]*telegraf.Tag, len(other.TagList())),
		fields:    make([]*telegraf.Field, len(other.FieldList())),
		tm:        other.Time(),
		tp:        other.Type(),
		aggregate: other.IsAggregate(),
	}

	for i, tag := range other.TagList() {
		m.tags[i] = &telegraf.Tag{Key: tag.Key, Value: tag.Value}
	}

	for i, field := range other.FieldList() {
		m.fields[i] = &telegraf.Field{Key: field.Key, Value: field.Value}
	}
	return m
}

func (m *metric) String() string {
	return fmt.Sprintf("%s %v %v %d", m.name, m.Tags(), m.Fields(), m.tm.UnixNano())
}

func (m *metric) Name() string {
	return m.name
}

func (m *metric) Tags() map[string]string {
	tags := make(map[string]string, len(m.tags))
	for _, tag := range m.tags {
		tags[tag.Key] = tag.Value
	}
	return tags
}

func (m *metric) TagList() []*telegraf.Tag {
	return m.tags
}

func (m *metric) Fields() map[string]interface{} {
	fields := make(map[string]interface{}, len(m.fields))
	for _, field := range m.fields {
		fields[field.Key] = field.Value
	}

	return fields
}

func (m *metric) FieldList() []*telegraf.Field {
	return m.fields
}

func (m *metric) Time() time.Time {
	return m.tm
}

func (m *metric) Type() telegraf.ValueType {
	return m.tp
}

func (m *metric) SetName(name string) {
	m.name = name
}

func (m *metric) AddPrefix(prefix string) {
	m.name = prefix + m.name
}

func (m *metric) AddSuffix(suffix string) {
	m.name = m.name + suffix
}

func (m *metric) AddTag(key, value string) {
	for i, tag := range m.tags {
		if key > tag.Key {
			continue
		}

		if key == tag.Key {
			tag.Value = value
			return
		}

		m.tags = append(m.tags, nil)
		copy(m.tags[i+1:], m.tags[i:])
		m.tags[i] = &telegraf.Tag{Key: key, Value: value}
		return
	}

	m.tags = append(m.tags, &telegraf.Tag{Key: key, Value: value})
}

func (m *metric) HasTag(key string) bool {
	for _, tag := range m.tags {
		if tag.Key == key {
			return true
		}
	}
	return false
}

func (m *metric) GetTag(key string) (string, bool) {
	for _, tag := range m.tags {
		if tag.Key == key {
			return tag.Value, true
		}
	}
	return "", false
}

func (m *metric) RemoveTag(key string) {
	for i, tag := range m.tags {
		if tag.Key == key {
			copy(m.tags[i:], m.tags[i+1:])
			m.tags[len(m.tags)-1] = nil
			m.tags = m.tags[:len(m.tags)-1]
			return
		}
	}
}

func (m *metric) AddField(key string, value interface{}) {
	for i, field := range m.fields {
		if key == field.Key {
			m.fields[i] = &telegraf.Field{Key: key, Value: convertField(value)}
			return
		}
	}
	m.fields = append(m.fields, &telegraf.Field{Key: key, Value: convertField(value)})
}

func (m *metric) HasField(key string) bool {
	for _, field := range m.fields {
		if field.Key == key {
			return true
		}
	}
	return false
}

func (m *metric) GetField(key string) (interface{}, bool) {
	for _, field := range m.fields {
		if field.Key == key {
			return field.Value, true
		}
	}
	return nil, false
}

func (m *metric) RemoveField(key string) {
	for i, field := range m.fields {
		if field.Key == key {
			copy(m.fields[i:], m.fields[i+1:])
			m.fields[len(m.fields)-1] = nil
			m.fields = m.fields[:len(m.fields)-1]
			return
		}
	}
}

func (m *metric) SetTime(t time.Time) {
	m.tm = t
}

func (m *metric) Copy() telegraf.Metric {
	m2 := &metric{
		name:      m.name,
		tags:      make([]*telegraf.Tag, len(m.tags)),
		fields:    make([]*telegraf.Field, len(m.fields)),
		tm:        m.tm,
		tp:        m.tp,
		aggregate: m.aggregate,
	}

	for i, tag := range m.tags {
		m2.tags[i] = &telegraf.Tag{Key: tag.Key, Value: tag.Value}
	}

	for i, field := range m.fields {
		m2.fields[i] = &telegraf.Field{Key: field.Key, Value: field.Value}
	}
	return m2
}

func (m *metric) SetAggregate(b bool) {
	m.aggregate = true
}

func (m *metric) IsAggregate() bool {
	return m.aggregate
}

func (m *metric) HashID() uint64 {
	h := fnv.New64a()
	h.Write([]byte(m.name))
	h.Write([]byte("\n"))
	for _, tag := range m.tags {
		h.Write([]byte(tag.Key))
		h.Write([]byte("\n"))
		h.Write([]byte(tag.Value))
		h.Write([]byte("\n"))
	}
	return h.Sum64()
}

func (m *metric) Accept() {
}

func (m *metric) Reject() {
}

func (m *metric) Drop() {
}

// Convert field to a supported type or nil if unconvertible
// tranfer to float64
func convertField(v interface{}) interface{} {
	switch v := v.(type) {
	case float64:
		return v
	case int64:
		return float64(v)
	case string:
		return atof(v)
	case bool:
		return btof(v)
	case int:
		return float64(v)
	case uint:
		return float64(v)
	case uint64:
		return float64(v)
	case []byte:
		return atof(string(v))
	case int32:
		return float64(v)
	case int16:
		return float64(v)
	case int8:
		return float64(v)
	case uint32:
		return float64(v)
	case uint16:
		return float64(v)
	case uint8:
		return float64(v)
	case float32:
		return float64(v)
	case *float64:
		if v != nil {
			return float64(*v)
		}
	case *int64:
		if v != nil {
			return float64(*v)
		}
	case *string:
		if v != nil {
			return atof(*v)
		}
	case *bool:
		if v != nil {
			return btof(*v)
		}
	case *int:
		if v != nil {
			return float64(*v)
		}
	case *uint:
		if v != nil {
			return float64(*v)
		}
	case *uint64:
		if v != nil {
			return float64(*v)
		}
	case *[]byte:
		if v != nil {
			return atof(string(*v))
		}
	case *int32:
		if v != nil {
			return float64(*v)
		}
	case *int16:
		if v != nil {
			return float64(*v)
		}
	case *int8:
		if v != nil {
			return float64(*v)
		}
	case *uint32:
		if v != nil {
			return float64(*v)
		}
	case *uint16:
		if v != nil {
			return float64(*v)
		}
	case *uint8:
		if v != nil {
			return float64(*v)
		}
	case *float32:
		if v != nil {
			return float64(*v)
		}
	default:
		return nil
	}
	return nil
}

func atof(s string) interface{} {
	if f, err := strconv.ParseFloat(s, 64); err != nil {
		return nil
	} else {
		return f
	}
}

func btof(b bool) interface{} {
	if b {
		return float64(1)
	}
	return float64(0)
}
