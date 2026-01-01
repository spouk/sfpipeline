package main

import (
	"bufio"
	"container/ring"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	BUFFER_SIZE    = 5
	CLEAR_INTERVAL = 30 * time.Second
)

var logger *log.Logger

func init() {
	logger = log.New(os.Stdout, "", log.Ldate|log.Ltime)
}

func logInfo(msg string, args ...interface{}) {
	logger.Printf("[INFO] "+msg, args...)
}

func logError(msg string, args ...interface{}) {
	logger.Printf("[ERROR] "+msg, args...)
}

// -- ring buffer
type RingBuffer struct {
	buffer *ring.Ring
	start  *ring.Ring
	size   int
	count  int
}

// make new instance
func NewRingBuffer(size int) *RingBuffer {
	r := ring.New(size)
	return &RingBuffer{
		buffer: r,
		start:  r,
		size:   size,
		count:  0,
	}
}

// push variable to ring buffer
func (rb *RingBuffer) Push(value int) {
	rb.buffer.Value = value
	rb.buffer = rb.buffer.Next()

	if rb.count < rb.size {
		rb.count++
	}
	logInfo("Добавлено в буфер: %d (всего: %d)", value, rb.count)
}

// получение значения с кольцевого буфера
func (rb *RingBuffer) GetValues() []int {
	var values []int
	if rb.count == 0 {
		logInfo("Буфер пуст")
		return values
	}

	current := rb.start
	for i := 0; i < rb.count; i++ {
		values = append(values, current.Value.(int))
		current = current.Next()
	}
	logInfo("Получено значений из буфера: %d", len(values))
	return values
}

// очистка кольцевого буфера
func (rb *RingBuffer) Clear() {
	newRing := ring.New(rb.size)
	rb.buffer = newRing
	rb.start = newRing
	rb.count = 0
	logInfo("Буфер очищен")
}

// -- pipeline

// (1) - фильтр отрицательных чисел
func filterNegative(input <-chan int) <-chan int {
	output := make(chan int)

	go func() {
		logInfo("Запуск фильтра отрицательных чисел")
		defer func() {
			close(output)
			logInfo("Завершение фильтра отрицательных чисел")
		}()

		for num := range input {
			if num >= 0 {
				logInfo("Фильтр отрицательных: пропущено %d", num)
				output <- num
			} else {
				logInfo("Фильтр отрицательных: отброшено %d", num)
			}
		}
	}()

	return output
}

// (2) - фильтр чисел, не кратных 3 (исключая 0)
func filterMultipleOf3(input <-chan int) <-chan int {
	output := make(chan int)

	go func() {
		logInfo("Запуск фильтра чисел, не кратных 3")
		defer func() {
			close(output)
			logInfo("Завершение фильтра чисел, не кратных 3")
		}()

		for num := range input {
			if num != 0 && num%3 == 0 {
				logInfo("Фильтр кратных 3: пропущено %d", num)
				output <- num
			} else {
				logInfo("Фильтр кратных 3: отброшено %d", num)
			}
		}
	}()

	return output
}

// (3) - буферизация с периодической очисткой
func bufferStage(input <-chan int) <-chan int {
	output := make(chan int)
	buffer := NewRingBuffer(BUFFER_SIZE)

	go func() {
		logInfo("Запуск стадии буферизации")
		defer func() {
			close(output)
			logInfo("Завершение стадии буферизации")
		}()

		ticker := time.NewTicker(CLEAR_INTERVAL)
		defer ticker.Stop()

		for {
			select {
			case num, ok := <-input:
				if !ok {
					logInfo("Входной канал закрыт")
					if vals := buffer.GetValues(); len(vals) > 0 {
						logInfo("Вывод остатков из буфера: %d значений", len(vals))
						for _, v := range vals {
							output <- v
						}
					}
					return
				}

				buffer.Push(num)

				if vals := buffer.GetValues(); len(vals) == BUFFER_SIZE {
					logInfo("Буфер полон, вывод данных")
					for _, v := range vals {
						output <- v
					}
					buffer.Clear()
				}

			case <-ticker.C:
				logInfo("Таймер: автоматическая очистка буфера")
				if vals := buffer.GetValues(); len(vals) > 0 {
					logInfo("Таймер: вывод %d значений", len(vals))
					for _, v := range vals {
						output <- v
					}
					buffer.Clear()
				}
			}
		}
	}()

	return output
}

// -- input source
func consoleSource() <-chan int {
	input := make(chan int)

	go func() {
		logInfo("Запуск источника данных (консоль)")
		defer func() {
			close(input)
			logInfo("Завершение источника данных")
		}()

		scanner := bufio.NewScanner(os.Stdin)

		fmt.Println("Вводите целые числа (для выхода введите 'exit'):")

		for {
			fmt.Print("> ")
			if !scanner.Scan() {
				logError("Ошибка чтения из консоли")
				break
			}

			text := strings.TrimSpace(scanner.Text())

			// Проверка на выход
			if strings.ToLower(text) == "exit" {
				logInfo("Получена команда выхода")
				return
			}

			// Фильтрация нечисловых данных
			num, err := strconv.Atoi(text)
			if err != nil {
				logInfo("Нечисловой ввод: %s", text)
				fmt.Println("Ошибка! Пожалуйста, введите целое число")
				continue
			}

			// Отправляем число в канал
			input <- num
			logInfo("Отправлено в пайплайн: %d", num)
		}
	}()

	return input
}

// -- consumer
func dataConsumer(output <-chan int) {
	logInfo("Запуск потребителя данных")
	for num := range output {
		logInfo("Потребитель получил: %d", num)
		fmt.Printf(">>> Получены данные: %d\n", num)
	}
	logInfo("Потребитель завершил работу")
}

// - main point
func main() {
	logInfo("=== ЗАПУСК ПАЙПЛАЙНА ===")
	fmt.Println("=== ПАЙПЛАЙН ОБРАБОТКИ ЧИСЕЛ ===")
	fmt.Printf("Настройки: размер буфера=%d, интервал очистки=%v\n",
		BUFFER_SIZE, CLEAR_INTERVAL)
	fmt.Println("Этапы обработки:")
	fmt.Println("1. Фильтр отрицательных чисел")
	fmt.Println("2. Фильтр чисел, не кратных 3 (исключая 0)")
	fmt.Println("3. Буферизация с периодической очисткой")
	fmt.Println("================================\n")

	// Создаем источник данных (консоль)
	source := consoleSource()

	// Строим пайплайн
	stage1 := filterNegative(source)
	stage2 := filterMultipleOf3(stage1)
	stage3 := bufferStage(stage2)

	// Запускаем потребителя
	dataConsumer(stage3)

	logInfo("=== ПРОГРАММА ЗАВЕРШЕНА ===")
	fmt.Println("Программа завершена!")
}
