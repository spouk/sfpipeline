//Напишите код, реализующий пайплайн, работающий с целыми числами и состоящий из следующих стадий:
//
//Стадия фильтрации отрицательных чисел (не пропускать отрицательные числа).
//Стадия фильтрации чисел, не кратных 3 (не пропускать такие числа), исключая также и 0.
//Стадия буферизации данных в кольцевом буфере с интерфейсом, соответствующим тому, который был дан в качестве задания в 19 модуле. В этой стадии предусмотреть опустошение буфера (и соответственно, передачу этих данных, если они есть, дальше) с определённым интервалом во времени. Значения размера буфера и этого интервала времени сделать настраиваемыми (как мы делали: через константы или глобальные переменные).
//Написать источник данных для конвейера. Непосредственным источником данных должна быть консоль.
//
//Также написать код потребителя данных конвейера. Данные от конвейера можно направить снова в консоль построчно, сопроводив их каким-нибудь поясняющим текстом, например: «Получены данные …».
//
//При написании источника данных подумайте о фильтрации нечисловых данных, которые можно ввести через консоль. Как и где их фильтровать, решайте сами.

package main

import (
	"bufio"
	"container/ring"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	BUFFER_SIZE    = 5
	CLEAR_INTERVAL = 30 * time.Second
)

// --  ring buffer
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
		start:  r, // Запоминаем начало
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
}

// получение значения с кольцевого буфера
func (rb *RingBuffer) GetValues() []int {
	var values []int
	// проверка на наличие "чегонить" в буфере
	if rb.count == 0 {
		return values
	}

	current := rb.start
	for i := 0; i < rb.count; i++ {
		values = append(values, current.Value.(int))
		current = current.Next()
	}
	return values
}

// очистка кольцевого буфера
func (rb *RingBuffer) Clear() {
	// Создаем новое пустое кольцо
	newRing := ring.New(rb.size)

	// Сбрасываем все указатели
	rb.buffer = newRing
	rb.start = newRing
	rb.count = 0

	fmt.Println("Буфер очищен")
}

// -- pipeline

// (1) -  фильтр отрицательных чисел
func filterNegative(input <-chan int) <-chan int {
	output := make(chan int)

	go func() {
		for num := range input {
			if num >= 0 {
				output <- num
			} else {
				fmt.Printf("[f1] Отброшено отрицательное число: %d\n", num)
			}
		}
		close(output)
	}()

	return output
}

// (2) -  фильтр чисел, не кратных 3 (исключая 0)
func filterMultipleOf3(input <-chan int) <-chan int {
	output := make(chan int)

	go func() {
		for num := range input {
			if num != 0 && num%3 == 0 {
				output <- num
			} else {
				fmt.Printf("[f2] Отброшено (не кратно 3 или 0): %d\n", num)
			}
		}
		close(output)
	}()

	return output
}

// (3) -  буферизация с периодической очисткой
func bufferStage(input <-chan int) <-chan int {
	output := make(chan int)
	buffer := NewRingBuffer(BUFFER_SIZE)

	go func() {
		defer close(output)

		// Таймер для периодической очистки
		ticker := time.NewTicker(CLEAR_INTERVAL)
		defer ticker.Stop()

		for {
			select {
			case num, ok := <-input:
				if !ok {
					// Входной канал закрыт, выводим остатки
					if vals := buffer.GetValues(); len(vals) > 0 {
						fmt.Println("\n[Завершение] Вывод остатков из буфера:")
						for _, v := range vals {
							output <- v
						}
					}
					return
				}

				// Добавляем в буфер
				buffer.Push(num)
				fmt.Printf("[Буфер] Добавлено: %d\n", num)

				// Проверяем, не заполнен ли буфер
				if vals := buffer.GetValues(); len(vals) == BUFFER_SIZE {
					fmt.Println("\n[Буфер полон] Вывод данных:")
					for _, v := range vals {
						output <- v
					}
					buffer.Clear()
				}

			case <-ticker.C:
				// Время очистить буфер
				if vals := buffer.GetValues(); len(vals) > 0 {
					fmt.Println("\n[Таймер] Автоматическая очистка буфера:")
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
		defer close(input)

		scanner := bufio.NewScanner(os.Stdin)

		fmt.Println("Вводите целые числа (для выхода введите 'exit'):")

		for {
			fmt.Print("> ")
			if !scanner.Scan() {
				break
			}

			text := strings.TrimSpace(scanner.Text())

			// Проверка на выход
			if strings.ToLower(text) == "exit" {
				fmt.Println("Завершение ввода...")
				return
			}

			// Фильтрация нечисловых данных
			num, err := strconv.Atoi(text)
			if err != nil {
				fmt.Println("Ошибка! Пожалуйста, введите целое число")
				continue
			}

			// Отправляем число в канал
			input <- num
			fmt.Printf("Отправлено в пайплайн: %d\n", num)
		}
	}()

	return input
}

// -- consumer
func dataConsumer(output <-chan int) {
	for num := range output {
		fmt.Printf(">>> Получены данные: %d\n", num)
	}
	fmt.Println("Пайплайн завершил работу!")
}

// - main point
func main() {
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
	stage1 := filterNegative(source)    // Фильтр 1
	stage2 := filterMultipleOf3(stage1) // Фильтр 2
	stage3 := bufferStage(stage2)       // Буферизация

	// Запускаем потребителя
	dataConsumer(stage3)

	fmt.Println("Программа завершена!")
}
