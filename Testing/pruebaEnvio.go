package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
)

type Maquina_virtual struct {
	Uuid              string
	Nombre            string
	Sistema_operativo string
	Memoria           int
	Cpu               int
	Estado            string
	Hostname          string
	Ip                string
	Persona_email     string
	Host_id           string
}

func main() {
	// Inicia una goroutine para enviar mensajes JSON cada segundo.
	//go sendMessages()
	//createVMTest()
	//modifyVMTest()
	deleteVMTest()
	//startVMTest()
	//stopVMTest()

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

func enviarMensaje(message []byte, url string) {

	// Enviamos el mensaje JSON al servidor mediante una solicitud POST.
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(message))
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

func createVMTest() {
	// Datos del mensaje JSON que queremos enviar al servidor.
	message := Maquina_virtual{
		Nombre:            "UqCloudTest",
		Sistema_operativo: "Debian_64",
		Memoria:           2048,
		Cpu:               3,
		Persona_email:     "jslopezd@uqvirtual.edu.co",
	}

	messageJSON, err := json.Marshal(message)
	if err != nil {
		fmt.Println("Error al codificar el mensaje JSON:", err)
		return
	}

	// URL del servidor al que enviaremos el mensaje.
	serverURL := "http://localhost:8081/json/createVirtualMachine"

	enviarMensaje(messageJSON, serverURL)

}

func modifyVMTest() {
	// Datos del mensaje JSON que queremos enviar al servidor.
	message := Maquina_virtual{
		Nombre:            "UqCloudTest",
		Sistema_operativo: "Debian_64",
		Memoria:           512,
		Cpu:               2,
		Persona_email:     "jslopezd@uqvirtual.edu.co",
	}

	// Crear un mapa que incluye el campo tipo_solicitud y el objeto Specifications
	payload := map[string]interface{}{
		"tipo_solicitud": "modify",
		"specifications": message,
	}

	messageJSON, err := json.Marshal(payload)
	if err != nil {
		fmt.Println("Error al codificar el mensaje JSON:", err)
		return
	}

	// URL del servidor al que enviaremos el mensaje.
	serverURL := "http://localhost:8081/json/modifyVM"

	enviarMensaje(messageJSON, serverURL)
}

func deleteVMTest() {

	// Crear un mapa que incluye el campo tipo_solicitud y el objeto Specifications
	payload := map[string]interface{}{
		"tipo_solicitud": "delete",
		"nombreVM":       "UqCloudTest",
	}

	messageJSON, err := json.Marshal(payload)
	if err != nil {
		fmt.Println("Error al codificar el mensaje JSON:", err)
		return
	}

	// URL del servidor al que enviaremos el mensaje.
	serverURL := "http://localhost:8081/json/deleteVM"

	enviarMensaje(messageJSON, serverURL)
}

func startVMTest() {

	// Crear un mapa que incluye el campo tipo_solicitud y el objeto Specifications
	payload := map[string]interface{}{
		"tipo_solicitud": "start",
		"nombreVM":       "UqCloudTest",
	}

	messageJSON, err := json.Marshal(payload)
	if err != nil {
		fmt.Println("Error al codificar el mensaje JSON:", err)
		return
	}

	// URL del servidor al que enviaremos el mensaje.
	serverURL := "http://localhost:8081/json/startVM"

	enviarMensaje(messageJSON, serverURL)
}

func stopVMTest() {

	// Crear un mapa que incluye el campo tipo_solicitud y el objeto Specifications
	payload := map[string]interface{}{
		"tipo_solicitud": "stop",
		"nombreVM":       "UqCloudTest",
	}

	messageJSON, err := json.Marshal(payload)
	if err != nil {
		fmt.Println("Error al codificar el mensaje JSON:", err)
		return
	}

	// URL del servidor al que enviaremos el mensaje.
	serverURL := "http://localhost:8081/json/stopVM"

	enviarMensaje(messageJSON, serverURL)
}
