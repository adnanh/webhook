// Copyright 2012-2016, 2019 Charles Banning. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file

package mxj

// ---------------- expose Map methods to MapSeq type ---------------------------

// Pretty print a Map.
func (msv MapSeq) StringIndent(offset ...int) string {
	return writeMap(map[string]interface{}(msv), true, true, offset...)
}

// Pretty print a Map without the value type information - just key:value entries.
func (msv MapSeq) StringIndentNoTypeInfo(offset ...int) string {
	return writeMap(map[string]interface{}(msv), false, true, offset...)
}

