package lookup

import "strings"

// Aliases contain public window settings only. Counters, names, masks and
// imagery come exclusively from the same validated capture as their source.
func parseMeterAlias(base []string, line string) *NativeMeter {
	p := strings.Split(line, "|")
	if len(p) != 19 || len(base) != 40 {
		return nil
	}
	h := append([]string(nil), base...)
	h[31] = strings.TrimPrefix(p[0], "@")
	for i, j := range []int{1, 2, 10, 11, 12, 13, 15, 16, 17, 18, 19, 32, 33, 34, 35, 36, 37, 38} {
		h[j] = p[i+1]
	}
	s := parseNativeMeter([]byte(strings.Join(h, "|") + "\n1|" + h[4] + "|0|Player|0"))
	if s.Error != "" || s.NativeMeter == nil || s.NativeMeter.Frame == nil {
		return nil
	}
	return s.NativeMeter
}

func expandMeterAliases(s BridgeSample) []BridgeSample {
	n := s.NativeMeter
	if n == nil || len(n.Aliases) == 0 {
		return []BridgeSample{s}
	}
	out := []BridgeSample{s}
	for _, a := range n.Aliases {
		copy := *n
		copy.ID, copy.Serial, copy.Frame = a.ID, a.Serial, a.Frame
		copy.Aliases = nil
		copy.Members = append([]NativeMeterMember(nil), n.Members...)
		if n.View != nil {
			v := *n.View
			v.Rows = append([]NativeViewRow(nil), n.View.Rows...)
			v.Choices = append([]NativeViewChoice(nil), n.View.Choices...)
			copy.View = &v
		}
		v := s
		v.NativeMeter = &copy
		v.MeterSlot = a.Frame.WindowID - 1
		v.MeterCaptureSlot = s.MeterSlot + 1
		out = append(out, v)
	}
	return out
}

func meterCaptureSlot(s BridgeSample) int {
	if s.MeterCaptureSlot > 0 {
		return s.MeterCaptureSlot - 1
	}
	return s.MeterSlot
}

func meterSampleForWindow(s BridgeSample, id int) BridgeSample {
	if s.NativeMeter == nil || s.NativeMeter.ControlOnly {
		return s
	}
	for _, v := range expandMeterAliases(s) {
		if v.NativeMeter.Frame != nil && v.NativeMeter.Frame.WindowID == id {
			return v
		}
	}
	s.NativeMeter = nil
	s.Error = "meter_shared_window_unavailable"
	return s
}
