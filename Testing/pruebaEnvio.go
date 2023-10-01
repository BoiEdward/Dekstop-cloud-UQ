package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
)

type Specifications struct {
	Name   string `json:"nombre"`
	OSType string `json:"tipoSO"`
	Memory int    `json:"memoria"`
	CPU    int    `json:"cpu"`
}

func main() {
	// Inicia una goroutine para enviar mensajes JSON cada segundo.
	//go sendMessages()
	enviarMensaje()

	// Espera una señal de cierre (Ctrl+C) para detener el programa.
	//<-make(chan struct{})
	//fmt.Println("Apagando el programa...")
}

/*func sendMessages() {
	for {
		// Seed el generador de números aleatorios con una semilla única.
		rand.Seed(time.Now().UnixNano())
		// Genera un número aleatorio en el rango de 1 a 10 para la memoria.
		randomMemory := (rand.Intn(9) + 1) * 100
		// Genera un número aleatorio en el rango de 1 a 5 para la CPU.
		randomCPU := (rand.Intn(5) + 1)

		// Seed el generador de números aleatorios con una semilla única.
		rand.Seed(time.Now().UnixNano())
		// Lista de letras que se utilizarán para generar nombres aleatorios.
		letters := "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
		// Genera un nombre aleatorio de longitud aleatoria (entre 5 y 10 letras).
		nameLength := rand.Intn(6) + 5
		name := ""

		for i := 0; i < nameLength; i++ {
			// Elige una letra aleatoria de la lista.
			randomIndex := rand.Intn(len(letters))
			randomLetter := letters[randomIndex]

			// Agrega la letra al nombre.
			name += string(randomLetter)
		}

		nombre := "Machine_" + name

		// Datos del mensaje JSON que queremos enviar al servidor.
		message := Specifications{
			Name:   nombre,
			OSType:   "Debian",
			Memory: randomMemory,
			CPU:    randomCPU,
		}

		messageJSON, err := json.Marshal(message)
		if err != nil {
			fmt.Println("Error al codificar el mensaje JSON:", err)
			return
		}

		// URL del servidor al que enviaremos el mensaje.
		serverURL := "http://localhost:8081/json/specifications"

		// Enviamos el mensaje JSON al servidor mediante una solicitud POST.
		resp, err := http.Post(serverURL, "application/json", bytes.NewBuffer(messageJSON))
		if err != nil {
			fmt.Println("Error al enviar la solicitud al servidor:", err)
			return
		}
		defer resp.Body.Close()

		// Verificamos la respuesta del servidor.
		if resp.StatusCode == http.StatusOK {
			fmt.Println("Mensaje enviado exitosamente al servidor.")
		} else {
			fmt.Println("Error al enviar el mensaje al servidor. Código de estado:", resp.StatusCode)
		}

		responseBody, err := ioutil.ReadAll(resp.Body)
		fmt.Println("Respuesta del servidor: " + string(responseBody))

		// Espera un segundo antes de enviar el siguiente mensaje.
		time.Sleep(1 * time.Second)
	}
}*/

func enviarMensaje() {
	// Datos del mensaje JSON que queremos enviar al servidor.
	message := Specifications{
		Name:   "UqCloud",
		OSType: "Debian",
		Memory: 1024,
		CPU:    2,
	}

	messageJSON, err := json.Marshal(message)
	if err != nil {
		fmt.Println("Error al codificar el mensaje JSON:", err)
		return
	}

	// URL del servidor al que enviaremos el mensaje.
	//serverURL := "http://localhost:8081/json/specifications"
	serverURL := "http://localhost:8081/json/modifyVM"

	// Enviamos el mensaje JSON al servidor mediante una solicitud POST.
	resp, err := http.Post(serverURL, "application/json", bytes.NewBuffer(messageJSON))
	if err != nil {
		fmt.Println("Error al enviar la solicitud al servidor:", err)
		return
	}
	defer resp.Body.Close()

	// Verificamos la respuesta del servidor.
	if resp.StatusCode == http.StatusOK {
		fmt.Println("Mensaje enviado exitosamente al servidor.")
	} else {
		fmt.Println("Error al enviar el mensaje al servidor. Código de estado:", resp.StatusCode)
	}

	responseBody, err := ioutil.ReadAll(resp.Body)
	fmt.Println("Respuesta del servidor: " + string(responseBody))
}
