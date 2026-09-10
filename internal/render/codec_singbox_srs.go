package render

import (
	"bufio"
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"fmt"
	"net/netip"

	"github.com/fl0w1nd/proxy-rule-manager/internal/ir"
	"github.com/sagernet/sing/common/domain"
	"github.com/sagernet/sing/common/varbin"
	"go4.org/netipx"
)

// This codec writes the SRS container by hand instead of calling
// github.com/sagernet/sing-box/common/srs. That package does the same job, but
// depending on sing-box pulls its module graph into PRM and pins an older
// github.com/sagernet/sing than the one PRM already uses. The encoder is kept
// in-tree and pinned to sing-box's own output by the fixtures in testdata.

var srsMagic = [3]byte{0x53, 0x52, 0x53}

const (
	srsRuleDefault uint8 = 0
	srsRuleLogical uint8 = 1

	srsItemNetwork          uint8 = 1
	srsItemDomain           uint8 = 2
	srsItemDomainKeyword    uint8 = 3
	srsItemDomainRegex      uint8 = 4
	srsItemSourceIPCIDR     uint8 = 5
	srsItemIPCIDR           uint8 = 6
	srsItemSourcePort       uint8 = 7
	srsItemSourcePortRange  uint8 = 8
	srsItemPort             uint8 = 9
	srsItemPortRange        uint8 = 10
	srsItemProcessName      uint8 = 11
	srsItemProcessPath      uint8 = 12
	srsItemProcessPathRegex uint8 = 17
	srsItemFinal            uint8 = 0xFF
)

// srsFields lists the rule fields this codec can encode, keyed by the sing-box
// field name. A condition mapped to any other field name has no SRS item type;
// dropping it silently would widen the rule, so it is reported instead.
var srsFields = map[string]bool{
	"network":            true,
	"domain":             true,
	"domain_suffix":      true,
	"domain_keyword":     true,
	"domain_regex":       true,
	"source_ip_cidr":     true,
	"ip_cidr":            true,
	"source_port":        true,
	"source_port_range":  true,
	"port":               true,
	"port_range":         true,
	"process_name":       true,
	"process_path":       true,
	"process_path_regex": true,
}

// renderSingboxSRS renders entries as a sing-box binary rule-set (SRS).
func renderSingboxSRS(tmpl *Template, entries []ir.Entry) ([]byte, error) {
	rules, err := buildSingboxRules(tmpl, entries)
	if err != nil {
		return nil, err
	}
	if len(rules) == 0 {
		return nil, nil
	}
	if err := checkSRSFields(rules); err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	if _, err := buf.Write(srsMagic[:]); err != nil {
		return nil, err
	}
	if err := binary.Write(&buf, binary.BigEndian, singboxRuleSetVersion); err != nil {
		return nil, err
	}
	compress, err := zlib.NewWriterLevel(&buf, zlib.BestCompression)
	if err != nil {
		return nil, err
	}
	w := bufio.NewWriter(compress)
	if _, err := varbin.WriteUvarint(w, uint64(len(rules))); err != nil {
		return nil, err
	}
	for i, rule := range rules {
		if err := writeSRSRule(w, rule); err != nil {
			return nil, fmt.Errorf("write rule[%d]: %w", i, err)
		}
	}
	if err := w.Flush(); err != nil {
		return nil, err
	}
	if err := compress.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func checkSRSFields(rules []singboxRule) error {
	for _, rule := range rules {
		for field := range rule.Fields {
			if !srsFields[field] {
				return fmt.Errorf("field %q cannot be encoded in an SRS rule-set", field)
			}
		}
		for field := range rule.Ports {
			if !srsFields[field] {
				return fmt.Errorf("field %q cannot be encoded in an SRS rule-set", field)
			}
		}
		if err := checkSRSFields(rule.Subs); err != nil {
			return err
		}
	}
	return nil
}

func writeSRSRule(w varbin.Writer, rule singboxRule) error {
	if rule.Mode != "" {
		return writeSRSLogical(w, rule)
	}
	return writeSRSDefault(w, rule)
}

func writeSRSLogical(w varbin.Writer, rule singboxRule) error {
	if err := w.WriteByte(srsRuleLogical); err != nil {
		return err
	}
	// sing-box encodes the logical mode as 0 for and, 1 for or. Any other mode
	// is a bug in buildLogicalSingboxRule, not something to guess at.
	var mode byte
	switch rule.Mode {
	case string(ir.KindAnd):
		mode = 0
	case string(ir.KindOr):
		mode = 1
	default:
		return fmt.Errorf("unsupported logical mode %q", rule.Mode)
	}
	if err := w.WriteByte(mode); err != nil {
		return err
	}
	if _, err := varbin.WriteUvarint(w, uint64(len(rule.Subs))); err != nil {
		return err
	}
	for i, sub := range rule.Subs {
		if err := writeSRSRule(w, sub); err != nil {
			return fmt.Errorf("sub-rule[%d]: %w", i, err)
		}
	}
	return binary.Write(w, binary.BigEndian, rule.Invert)
}

// writeSRSDefault writes a condition-only rule. The item order is fixed by the
// SRS format, so it follows sing-box's writer sequence rather than the field map.
func writeSRSDefault(w varbin.Writer, rule singboxRule) error {
	if err := w.WriteByte(srsRuleDefault); err != nil {
		return err
	}
	if err := writeSRSStrings(w, srsItemNetwork, rule.Fields["network"]); err != nil {
		return err
	}
	domains, suffixes := rule.Fields["domain"], rule.Fields["domain_suffix"]
	if len(domains) > 0 || len(suffixes) > 0 {
		if err := w.WriteByte(srsItemDomain); err != nil {
			return err
		}
		if err := domain.NewMatcher(domains, suffixes, false).Write(w); err != nil {
			return fmt.Errorf("domain matcher: %w", err)
		}
	}
	if err := writeSRSStrings(w, srsItemDomainKeyword, rule.Fields["domain_keyword"]); err != nil {
		return err
	}
	if err := writeSRSStrings(w, srsItemDomainRegex, rule.Fields["domain_regex"]); err != nil {
		return err
	}
	if err := writeSRSCIDR(w, srsItemSourceIPCIDR, rule.Fields["source_ip_cidr"]); err != nil {
		return err
	}
	if err := writeSRSCIDR(w, srsItemIPCIDR, rule.Fields["ip_cidr"]); err != nil {
		return err
	}
	if err := writeSRSUint16(w, srsItemSourcePort, rule.Ports["source_port"]); err != nil {
		return err
	}
	if err := writeSRSStrings(w, srsItemSourcePortRange, rule.Fields["source_port_range"]); err != nil {
		return err
	}
	if err := writeSRSUint16(w, srsItemPort, rule.Ports["port"]); err != nil {
		return err
	}
	if err := writeSRSStrings(w, srsItemPortRange, rule.Fields["port_range"]); err != nil {
		return err
	}
	if err := writeSRSStrings(w, srsItemProcessName, rule.Fields["process_name"]); err != nil {
		return err
	}
	if err := writeSRSStrings(w, srsItemProcessPath, rule.Fields["process_path"]); err != nil {
		return err
	}
	if err := writeSRSStrings(w, srsItemProcessPathRegex, rule.Fields["process_path_regex"]); err != nil {
		return err
	}
	if err := w.WriteByte(srsItemFinal); err != nil {
		return err
	}
	return binary.Write(w, binary.BigEndian, rule.Invert)
}

func writeSRSStrings(w varbin.Writer, itemType uint8, values []string) error {
	if len(values) == 0 {
		return nil
	}
	if err := w.WriteByte(itemType); err != nil {
		return err
	}
	if _, err := varbin.WriteUvarint(w, uint64(len(values))); err != nil {
		return err
	}
	for _, s := range values {
		if _, err := varbin.WriteUvarint(w, uint64(len(s))); err != nil {
			return err
		}
		if _, err := w.Write([]byte(s)); err != nil {
			return err
		}
	}
	return nil
}

func writeSRSUint16(w varbin.Writer, itemType uint8, values []uint16) error {
	if len(values) == 0 {
		return nil
	}
	if err := w.WriteByte(itemType); err != nil {
		return err
	}
	if _, err := varbin.WriteUvarint(w, uint64(len(values))); err != nil {
		return err
	}
	return binary.Write(w, binary.BigEndian, values)
}

func writeSRSCIDR(w varbin.Writer, itemType uint8, values []string) error {
	if len(values) == 0 {
		return nil
	}
	var builder netipx.IPSetBuilder
	for i, raw := range values {
		if prefix, err := netip.ParsePrefix(raw); err == nil {
			builder.AddPrefix(prefix)
			continue
		}
		addr, err := netip.ParseAddr(raw)
		if err != nil {
			return fmt.Errorf("parse CIDR [%d]: %w", i, err)
		}
		builder.Add(addr)
	}
	set, err := builder.IPSet()
	if err != nil {
		return err
	}
	ranges := set.Ranges()

	if err := w.WriteByte(itemType); err != nil {
		return err
	}
	if err := w.WriteByte(1); err != nil {
		return err
	}
	if err := binary.Write(w, binary.BigEndian, uint64(len(ranges))); err != nil {
		return err
	}
	for _, ipRange := range ranges {
		if err := writeSRSAddr(w, ipRange.From()); err != nil {
			return err
		}
		if err := writeSRSAddr(w, ipRange.To()); err != nil {
			return err
		}
	}
	return nil
}

func writeSRSAddr(w varbin.Writer, addr netip.Addr) error {
	b := addr.AsSlice()
	if _, err := varbin.WriteUvarint(w, uint64(len(b))); err != nil {
		return err
	}
	_, err := w.Write(b)
	return err
}
