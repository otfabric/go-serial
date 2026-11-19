// SPDX-License-Identifier: MIT

package serial

// The Win32 communications API's values that the Windows backend maps the
// config and the modem lines to. They are plain numbers, kept here without a
// build constraint so that the mapping is tested on every platform.

// Bits of the DCB's bit-field DWORD (fBinary to fAbortOnError).
const (
	dcbBinary          = 1 << 0  // fBinary
	dcbParity          = 1 << 1  // fParity
	dcbOutxCtsFlow     = 1 << 2  // fOutxCtsFlow
	dcbOutxDsrFlow     = 1 << 3  // fOutxDsrFlow
	dcbDtrHandshake    = 2 << 4  // fDtrControl = DTR_CONTROL_HANDSHAKE
	dcbOutX            = 1 << 8  // fOutX
	dcbInX             = 1 << 9  // fInX
	dcbRtsHandshake    = 2 << 12 // fRtsControl = RTS_CONTROL_HANDSHAKE
	dcbDefaultXonLimit = 2048    // XonLim when the driver reports none
	dcbDefaultXoffLim  = 512     // XoffLim when the driver reports none
)

// EscapeCommFunction functions.
const (
	commSetRTS = 3 // SETRTS
	commClrRTS = 4 // CLRRTS
	commSetDTR = 5 // SETDTR
	commClrDTR = 6 // CLRDTR
)

// GetCommModemStatus bits.
const (
	commCTSOn  = 0x0010 // MS_CTS_ON
	commDSROn  = 0x0020 // MS_DSR_ON
	commRingOn = 0x0040 // MS_RING_ON
	commRLSDOn = 0x0080 // MS_RLSD_ON
)

// dcbFlags returns the DCB's bit-field DWORD for c: binary mode, the parity
// check and the flow control.
func dcbFlags(c *Config) uint32 {
	flags := uint32(dcbBinary)
	if c.Parity == ParityEven || c.Parity == ParityOdd {
		flags |= dcbParity
	}
	switch c.FlowControl {
	case FlowNone:
	case FlowRTSCTS:
		flags |= dcbOutxCtsFlow | dcbRtsHandshake
	case FlowDTRDSR:
		flags |= dcbOutxDsrFlow | dcbDtrHandshake
	case FlowXONXOFF:
		flags |= dcbOutX | dcbInX
	}
	return flags
}

// commEscape returns the EscapeCommFunction function that asserts (on) or
// clears RTS (rts) or DTR.
func commEscape(rts, on bool) uint32 {
	switch {
	case rts && on:
		return commSetRTS
	case rts:
		return commClrRTS
	case on:
		return commSetDTR
	}
	return commClrDTR
}

// commModemStatus returns the status inputs set in GetCommModemStatus bits.
func commModemStatus(bits uint32) ModemStatus {
	return ModemStatus{
		CTS: bits&commCTSOn != 0,
		DSR: bits&commDSROn != 0,
		RI:  bits&commRingOn != 0,
		DCD: bits&commRLSDOn != 0,
	}
}
