# Copyright 2011 The Go Authors. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.

include $(GOROOT)/src/Make.inc

TARG=goissue
GOFILES=\
	goissue.go\

include $(GOROOT)/src/Make.cmd
