package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"sync"
	"sync/atomic"
)

// Interactive helper to find a solution to the game "Primel" (https://converged.yt/primel/)
// The helper uses the simple heuristics of finding the most restrictive digit for each position
// It is not optimal! A better helper would find the most restrictive guess considering all
// remaining candidates. The optimal solver would find the best guess considering all possible
// outcomes for each candidate.
func main() {
	// Calculate set of possible values
	candidates := getPrimes(10000, 100000)

	// Find best guess according to the frequency of each digit per position
	bestGuess := findBestGuess(candidates)
	fmt.Printf("The best first guess is: %05d. The number of remaining candidates is %v\n", bestGuess, len(candidates))

	// Incorporate feedback and find next best guess
	for {
		feedbackPerDigit := readFeedbackForDigits(getDigits(bestGuess, 5))
		if all(feedbackPerDigit, func(f feedback) bool { return f.feedbackType == feedbackTypeCorrect }) {
			fmt.Printf("We found the correct number (\033[32m\033[1m%v\033[0m)! 🎉\n", bestGuess)
			break
		}
		candidates = incorporateFeedback(feedbackPerDigit, candidates)
		if len(candidates) == 0 {
			fmt.Fprintf(os.Stderr, "No more candidates found!")
			os.Exit(1)
		}
		bestGuess = findBestGuess(candidates)
		fmt.Printf("The new best guess is: %05d. The number of remaining candidates is %v\n", bestGuess, len(candidates))
	}
}

func findBestGuess(candidates []uint) uint {
	var bestGuess uint
	var bestGuessValue uint64
	for i := 0; i < len(candidates); i++ {
		candidateGuess := candidates[i]
		guessValue := evaluateGuess(candidateGuess, candidates)
		if guessValue > bestGuessValue {
			bestGuess = candidateGuess
		}
	}
	return bestGuess
}

func evaluateGuess(guess uint, candidates []uint) uint64 {
	var remainingCandidatesAfterGuess uint64
	var wg sync.WaitGroup
	for i := 0; i < len(candidates); i++ {
		possibleSolution := candidates[i]
		wg.Add(1)
		go func() {
			feedbackPerDigit := getFeedbackPerDigit(getDigits(guess, 5), getDigits(possibleSolution, 5))
			newCandidates := incorporateFeedback(feedbackPerDigit, candidates)
			atomic.AddUint64(&remainingCandidatesAfterGuess, uint64(len(newCandidates)))
			wg.Done()
		}()
	}
	wg.Wait()
	return remainingCandidatesAfterGuess
}

func getFeedbackPerDigit(guessDigits []uint, possibleSolutionDigits []uint) []feedback {
	if len(guessDigits) != len(possibleSolutionDigits) {
		panic("The length of the guess and the solution must be the same!")
	}
	feedbackPerDigit := make([]feedback, len(guessDigits))

	// Handle all correct digits first
	for i := 0; i < len(guessDigits); i++ {
		if guessDigits[i] == possibleSolutionDigits[i] {
			feedbackPerDigit[i] = feedback{feedbackType: feedbackTypeCorrect, digit: guessDigits[i]}
		}
	}

	// Handle remaining (present and absent) digits
	for i := 0; i < len(guessDigits); i++ {
		// Correct digits have already been handled
		if feedbackPerDigit[i].feedbackType == feedbackTypeCorrect {
			continue
		}

		for j := 0; j < len(possibleSolutionDigits); j++ {
			// Don't consider already correct digits to determine if the current digit is present or
			// absent
			if feedbackPerDigit[j].feedbackType == feedbackTypeCorrect {
				continue
			} else if possibleSolutionDigits[j] == guessDigits[i] {
				feedbackPerDigit[i] = feedback{feedbackType: feedbackTypePresent, digit: guessDigits[i]}
				break
			} else if j == len(possibleSolutionDigits)-1 {
				feedbackPerDigit[i] = feedback{feedbackType: feedbackTypeAbsent, digit: guessDigits[i]}
			}
		}
	}

	return feedbackPerDigit
}

func incorporateFeedback(feedbackPerDigit []feedback, candidates []uint) (newCandidates []uint) {
	newCandidates = make([]uint, len(candidates))
	copy(newCandidates, candidates)
	var correctPositions []uint

	// Process correct feedbacks first as they affect the other feedbacks
	for i := 0; i < len(feedbackPerDigit); i++ {
		if feedbackPerDigit[i].feedbackType == feedbackTypeCorrect {
			correctPositions = append(correctPositions, uint(i))
			newCandidates = filter(newCandidates, func(candidate uint) bool {
				return getDigits(candidate, 5)[i] == feedbackPerDigit[i].digit
			})
		}
	}

	for i := 0; i < len(feedbackPerDigit); i++ {
		switch feedbackPerDigit[i].feedbackType {
		case feedbackTypeCorrect:
			// Already processed
			// Do nothing
		case feedbackTypePresent:
			newCandidates = filter(newCandidates, func(candidate uint) bool {
				for index, digit := range getDigits(candidate, 5) {
					if digit == feedbackPerDigit[i].digit && index != i && !contains(correctPositions, uint(index)) {
						return true
					}
				}
				return false
			})
		case feedbackTypeAbsent:
			newCandidates = filter(newCandidates, func(candidate uint) bool {
				for index, digit := range getDigits(candidate, 5) {
					if digit == feedbackPerDigit[i].digit && !contains(correctPositions, uint(index)) {
						return false
					}
				}
				return true
			})
		default:
			fmt.Fprintf(os.Stderr, "Unknown feedback type")
			os.Exit(2)
		}
	}
	return
}

func getPrimes(from uint, to uint) []uint {
	primesTo := sieve(to)
	var result []uint
	for i := 0; i < len(primesTo); i++ {
		if primesTo[i] >= from {
			result = append(result, primesTo[i])
		}
	}
	return result
}

func sieve(max uint) []uint {
	if max < 2 {
		return []uint{}
	}

	var primes []uint
	// Generate a list of all candidates where the value of the candidate the index + 2 and the
	// boolean flag determines if a prime candidate is valid or not
	candidates := make([]bool, max-2)
	for i := 0; i < len(candidates); i++ {
		candidates[i] = true
	}
	// Iterate over the prime candidates and invalidate multiples of each
	for i := 0; i < len(candidates); i++ {
		if candidates[i] {
			primes = append(primes, uint(i+2))
			// (i+2) is the value of the prime candidate
			// (i+2) * 2 is 2 * the value of the prime candidate
			// (i+2) * 2 - 2 is the index of the first multiple of the prime candidate
			// This index is incremented by (i+2) to find the next multiple
			for j := (i+2)*2 - 2; j < len(candidates); j += i + 2 {
				candidates[j] = false
			}
		}
	}

	return primes
}

func getDigits(num uint, numberOfDigits uint) []uint {
	var result []uint
	for i := 0; i < int(numberOfDigits); i++ {
		result = append(result, num%10)
		num /= 10
	}
	return result
}

type feedback struct {
	digit        uint
	feedbackType feedbackType
}

type feedbackType uint

const (
	feedbackTypeAbsent feedbackType = iota
	feedbackTypePresent
	feedbackTypeCorrect
)

func readFeedbackForDigits(guessDigits []uint) []feedback {
	result := make([]feedback, len(guessDigits))
	reader := bufio.NewReader(os.Stdin)
	for i := len(guessDigits) - 1; i >= 0; i-- {
		fmt.Printf("Was the digit in position \033[1m%v\033[0m of the guess (", len(guessDigits)-i)
		for j := len(guessDigits) - 1; j >= 0; j-- {
			if j == i {
				fmt.Printf("\033[4m\033[1m%v\033[0m", guessDigits[j])
			} else {
				fmt.Printf("\033[1m%v\033[0m", guessDigits[j])
			}
		}
		fmt.Printf(") in the \033[32mcorrect\033[0m position, \033[33mpresent\033[0m but in the wrong position or \033[31mabsent\033[0m? [\033[32mc\033[0m/\033[33mp\033[0m/\033[31ma\033[0m] ")
		for {
			// TODO: Handle possible errors while reading
			text, _ := reader.ReadString('\n')
			text = strings.TrimSuffix(text, "\n")
			switch text {
			case "c":
				result[i] = feedback{guessDigits[i], feedbackTypeCorrect}
			case "p":
				result[i] = feedback{guessDigits[i], feedbackTypePresent}
			case "a":
				result[i] = feedback{guessDigits[i], feedbackTypeAbsent}
			default:
				fmt.Fprintf(os.Stderr, "Invalid feedback: %s\n", text)
				continue
			}
			break
		}
	}
	return result
}

func filter(slice []uint, predicate func(uint) bool) []uint {
	var newSlice []uint
	for i := 0; i < len(slice); i++ {
		if predicate(slice[i]) {
			newSlice = append(newSlice, slice[i])
		}
	}
	return newSlice
}

func contains(slice []uint, elem uint) bool {
	for i := 0; i < len(slice); i++ {
		if slice[i] == elem {
			return true
		}
	}
	return false
}

func all(slice []feedback, predicate func(feedback) bool) bool {
	for i := 0; i < len(slice); i++ {
		if !predicate(slice[i]) {
			return false
		}
	}
	return true
}
