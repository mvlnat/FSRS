package handler

import (
	"fmt"
	"unicode/utf8"
)

const (
	maxDeckDescriptionLength = 100000
	maxCardContentLength     = 100000
	maxCardLinkLength        = 8192
	maxImportCardCount       = 10000
)

func validateDeckDescription(description string) error {
	if utf8.RuneCountInString(description) > maxDeckDescriptionLength {
		return fmt.Errorf("description must be %d characters or fewer", maxDeckDescriptionLength)
	}

	return nil
}

func validateDeckCardTemplates(frontTemplate, backTemplate string) error {
	if utf8.RuneCountInString(frontTemplate) > maxCardContentLength {
		return fmt.Errorf("front template must be %d characters or fewer", maxCardContentLength)
	}
	if utf8.RuneCountInString(backTemplate) > maxCardContentLength {
		return fmt.Errorf("back template must be %d characters or fewer", maxCardContentLength)
	}

	return nil
}

func validateCardContent(front, back, link string) error {
	if utf8.RuneCountInString(front) > maxCardContentLength {
		return fmt.Errorf("front must be %d characters or fewer", maxCardContentLength)
	}
	if utf8.RuneCountInString(back) > maxCardContentLength {
		return fmt.Errorf("back must be %d characters or fewer", maxCardContentLength)
	}
	if utf8.RuneCountInString(link) > maxCardLinkLength {
		return fmt.Errorf("link must be %d characters or fewer", maxCardLinkLength)
	}

	return nil
}

func validateImportCardCount(count int) error {
	if count > maxImportCardCount {
		return fmt.Errorf("deck import must contain %d cards or fewer", maxImportCardCount)
	}

	return nil
}
