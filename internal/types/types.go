// Copyright 2016 Tom Thorogood. All rights reserved.
// Use of this source code is governed by a
// Modified BSD License license that can be found in
// the LICENSE file.

package types

type IPBlockerAgent struct {
	IP4, IP6, IP6Route []byte
}
