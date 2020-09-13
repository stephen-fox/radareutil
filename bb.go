package radareutil

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"strings"
	"unicode"
)

// PdbToBasicBlockText formats the output of 'pdb' (print disassembly of
// a basic block) into a pretty formatted basic block. For example,
// assume the following 'pdb' output:
//             ;-- rip:
//            ; 0x100555b8c
//            ; "SecCertificateCopyValues"
//            488d351a8a1f.  lea rsi, qword str.SecCertificateCopyValues
//            4889c7         mov rdi, rax
//            e806200c00     call sym.imp.dlsym
//            4989c7         mov r15, rax
//            4d85ff         test r15, r15
//   ,=<      0f84fc000000   je 0x10035d282
//
// This would become:
//  ┌─────────────────────────────────────────────┐
//  │ ;-- rip:                                    │
//  │ ; 0x100555b8c                               │
//  │ ; "SecCertificateCopyValues"                │
//  │ lea rsi, qword str.SecCertificateCopyValues │
//  │ mov rdi, rax                                │
//  │ call sym.imp.dlsym                          │
//  │ mov r15, rax                                │
//  │ test r15, r15                               │
//  │ je 0x10035d282                              │
//  └─────────────────────────────────────────────┘
func PdbToBasicBlockText(reader io.Reader) (string, error) {
	var lines []string
	var maxLen int
	hasHeader := false
	inset := 0
	appendLineFn := func(line string) {
		lines = append(lines, line)
		if lineLen := len(line); lineLen > maxLen {
			maxLen = lineLen
		}
	}

	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		if !hasHeader {
			for i, c := range scanner.Bytes() {
				if !unicode.IsSpace(rune(c)) {
					inset = i
					hasHeader = true
					appendLineFn(scanner.Text()[i:])
					break
				}
			}
			continue
		}

		if len(scanner.Bytes()) == 0 {
			break
		}

		line := scanner.Text()
		if len(line) > inset {
			line = line[inset:]
		}
		if !unicode.IsNumber(rune(line[0])) && !unicode.IsLetter(rune(line[0])) {
			appendLineFn(line)
			continue
		}

		firstSpace := strings.Index(line, " ")
		if firstSpace < 0 {
			appendLineFn(line)
			continue
		}

		for i, c := range line[firstSpace:] {
			if unicode.IsSpace(c) {
				continue
			}

			line = line[firstSpace+i:]
			break
		}

		appendLineFn(line)
	}
	if scanner.Err() != nil {
		return "", scanner.Err()
	}

	buff := bytes.NewBuffer([]byte(fmt.Sprintf("┌%s┐\n", strings.Repeat("─", maxLen + 2))))
	for i := range lines {
		suffixPadding := maxLen - len(lines[i]) + 1
		buff.WriteString(fmt.Sprintf("│ %s%s│\n", lines[i], strings.Repeat(" ", suffixPadding)))
	}
	buff.WriteString(fmt.Sprintf("└%s┘", strings.Repeat("─", maxLen + 2)))

	return buff.String(), nil
}
