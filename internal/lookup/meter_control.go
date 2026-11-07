package lookup

import (
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"regexp"
	"strconv"
	"strings"
)

var meterHistoryScope = regexp.MustCompile(`^(history:([1-9]|1[0-9]|2[0-5]):(25|50|75|100|125|150|175|200):(all|boss|key)|savedu?:[0-9a-f]{32})$`)

// A focused native preview publishes only public identity and a short-lived
// request. This packet carries no damage counters and cannot become meter data.
func parseMeterControl(data []byte) (*NativeMeter, error) {
	bad := func() (*NativeMeter, error) { return nil, fmt.Errorf("meter_control_invalid") }
	if len(data) != 384 || string(data[:4]) != "LMC1" {
		return bad()
	}
	size := int(binary.BigEndian.Uint16(data[4:6]))
	if size < 1 || size > 374 {
		return bad()
	}
	body := data[10 : 10+size]
	if binary.BigEndian.Uint32(data[6:10]) != crc32.ChecksumIEEE(body) {
		return bad()
	}
	f := strings.Split(string(body), "|")
	if !((len(f) == 7 && f[0] == "1") || (len(f) == 11 && (f[0] == "2" || f[0] == "3")) || ((len(f) == 12 || len(f) == 14) && f[0] == "3") || (len(f) == 14 && (f[0] == "4" || f[0] == "5"))) || !reloadToken.MatchString(f[1]) || !observationGUID.MatchString(f[2]) {
		return bad()
	}
	expires, err := strconv.ParseInt(f[3], 10, 64)
	if err != nil || expires <= 0 || strconv.FormatInt(expires, 10) != f[3] {
		return bad()
	}
	if f[4] != "PARTY" && f[4] != "RAID" && f[4] != "INSTANCE_CHAT" && f[4] != "SELF" {
		return bad()
	}
	if f[0] == "5" {
		if f[5] != "navigation" || f[4] != "SELF" || f[6] != "0" {
			return bad()
		}
	} else if f[5] != "key" && f[5] != "boss" && f[5] != "boss-uptime" && !(f[0] == "4" && (meterWindowScope.MatchString(f[5]) || meterHistoryScope.MatchString(f[5]))) {
		return bad()
	}
	roster, err := strconv.ParseUint(f[6], 10, 32)
	if err != nil || strconv.FormatUint(roster, 10) != f[6] {
		return bad()
	}
	n := &NativeMeter{ControlOnly: true, ControlWidth: 64, ControlHeight: 8, ID: "share:" + f[2] + ":" + f[1], Player: f[2], ShareRequest: &MeterShareRequest{Token: f[1], Expires: expires, Channel: f[4], Scope: f[5], Roster: uint32(roster)}}
	if f[0] == "2" || f[0] == "3" || f[0] == "4" || f[0] == "5" {
		if (f[7] != "0" && f[7] != "1") || (f[8] != "request" && f[8] != "ready" && f[8] != "error") || (f[9] != "0" && f[9] != "1") {
			return bad()
		}
		received, err := strconv.Atoi(f[10])
		if err != nil || received < 0 || received > 13000 || strconv.Itoa(received) != f[10] {
			return bad()
		}
		n.ControlHidden = f[9] == "1"
		n.ShareRequest.TransferVersion, _ = strconv.Atoi(f[0])
		if len(f) >= 12 {
			n.ShareRequest.Compact = true
			option, e := strconv.Atoi(f[11])
			if e != nil || option < 0 || option > 7 || option&5 == 5 || strconv.Itoa(option) != f[11] {
				return bad()
			}
			n.ShareRequest.Options = uint8(option)
			if len(f) == 14 {
				end, e := strconv.ParseInt(f[12], 10, 64)
				if e != nil || end < 0 || strconv.FormatInt(end, 10) != f[12] {
					return bad()
				}
				id, e := strconv.Atoi(f[13])
				if e != nil || id < 0 || id > 1000000 || strconv.Itoa(id) != f[13] {
					return bad()
				}
				n.ShareRequest.EndedAt = end
				n.ShareRequest.EncounterID = f[13]
			}
		}
		n.ShareRequest.FocusBlocked = f[7] == "0"
		n.ShareRequest.ReceivedBytes = received
		if f[8] != "request" {
			n.ShareAck = f[1]
			n.NavigationFailed = f[0] == "5" && f[8] == "error"
			n.ShareRequest = nil
		}
	}
	return n, nil
}
