package resume

import "context"

type PDFParser struct{}

func NewPDFParser() *PDFParser {
	return &PDFParser{}
}

func (p *PDFParser) Parse(ctx context.Context, input ParseInput) (ParsedResume, error) {
	return ParsedResume{}, ErrParseFailed
}
