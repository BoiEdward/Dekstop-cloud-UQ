package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
)

type Maquina_virtual struct {
	Uuid                           string
	Nombre                         string
	Ram                            int
	Cpu                            int
	Ip                             string
	Estado                         string
	Hostname                       string
	Persona_email                  string
	Host_id                        int
	Disco_id                       int
	Sistema_operativo              string
	Distribucion_sistema_operativo string
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
		Nombre:                         "UqCloud Test",
		Sistema_operativo:              "Linux",
		Distribucion_sistema_operativo: "Debian",
		Ram:                            1024,
		Cpu:                            3,
		Persona_email:                  "jslopezd@uqvirtual.edu.co",
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
		Nombre:                         "UqCloud Test_Jhoiner",
		Sistema_operativo:              "Linux",
		Distribucion_sistema_operativo: "Debian_64",
		Ram:                            512,
		Cpu:                            1,
		Persona_email:                  "jslopezd@uqvirtual.edu.co",
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
		"nombreVM":       "Test_LjJf",
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
		"nombreVM":       "UqCloud Test_Jhoiner",
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
		"nombreVM":       "UqCloud Test_Jhoiner",
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
