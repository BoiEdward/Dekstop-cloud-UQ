package main

import (
	"container/list"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
	"golang.org/x/crypto/ssh"

	_ "github.com/go-sql-driver/mysql"
)

var db *sql.DB

// Variable que almacena la ruta de la llave privada ingresada por paametro cuando de ejecuta el programa
var privateKeyPath = flag.String("key", "", "Ruta de la llave privada SSH")

// Cola de especificaciones para la gestiòn de màquinas virtuales
// La gestiòn puede ser: modificar, eliminar, iniciar, detener una MV.
type ManagementQueue struct {
	sync.Mutex
	Queue *list.List
}

/*
Estrucutura de datos tipo JSON que contiene los campos necesarios para la gestiòn de usuarios
@Nombre Representa el nombre del usuario
@Apellido Representa el apellido del usuario
@Email Representa el email del usuario
@Usuario Representa el usuario de la cuenta con el cual puede iniciar sesiìn
@Contrasenia Representa la contraseña de la cuenta
*/
type Persona struct {
	Nombre      string
	Apellido    string
	Email       string
	Usuario     string
	Contrasenia string
}

/*
Estructura de datos tipo JSOn que contiene los datos de una màquina virtual
@Uuid Representa el uuid de una màqina virtual, el cual es un identificador ùnico
@Nombre Representa el nombre de la MV
@Sistema_operativo Representa el tipo de sistema operativo
@Memoria Representa la cantidad de memoria RAM de la MV
@Cpu Representa la cantidad de unidades de procesamiento de la MV
@Estado Representa el estado de la MV (Apagada, Encendida)
@Hostname Representa el nombre del host de la MV
@Ip Representa la direcciòn Ip de la MV
*/
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

type Maquina_virtualQueue struct {
	sync.Mutex
	Queue *list.List
}

/*
Estructura de datos tipo JSON que contiene los campos de un host
@Id Representa el identificador ùnico del host
@Nombre Representa el nombre del host
@Mac Representa la direcciòn fìsica del host
@Memoria Representa la cantidad de memoria RAM que tiene el host
@Cpu Representa la cantidad de unidades de procesamiento que tiene el host
@Adaptador_red Representa el nombre del adaptador de red
@Almacenamiento_total Representa el total de espacio de almacenamiento que tiene el host
@Estado Representa el estado del host (Disponible Fuera de servicio)
@Sistema_operativo Representa el tipo de sistema operativo del host
@Ruta_disco_multi Representa la ubiaciòn del disco multiconexiòn
@Ruta_llave_ssh Representa la ubiaciòn de la llave ssh pùblica
@Hostname Representa el nombre del host
@Ip Representa la direcciòn Ip del host
*/
type Host struct {
	Id                   int
	Nombre               string
	Mac                  string
	Memoria              int
	Cpu                  int
	Adaptador_red        string
	Almacenamiento_total int
	Estado               string
	Sistema_operativo    string
	Ruta_disco_multi     string
	Ruta_llave_ssh       string
	Hostname             string
	Ip                   string
}

/*
Estructura de datos tipo JSON que contiene los campos para representar una MV del catàlogo
@Nombre Representa el nombre de la MV
@Memoria Representa la cantidad de memoria RAM de la MV
@Cpu Representa la cantidad de unidades de procesamiento de la MV
@Sistema_operativo Representa el tipo de sistema operativo de la Mv
*/
type Catalogo struct {
	Nombre            string
	Memoria           int
	Cpu               int
	Sistema_operativo string
}

// Declaraciòn de variables globales
var (
	maquina_virtualesQueue Maquina_virtualQueue
	managementQueue        ManagementQueue
	mu                     sync.Mutex
	lastQueueSize          int
)

func main() {

	flag.Parse()

	//Verifica que el paràmetro de la ruta de la llave privada no estè vacìo
	if *privateKeyPath == "" {
		fmt.Println("Debe ingresar la ruta de la llave privada SSH")
		return
	}

	// Conexión a SQL
	manageSqlConecction()

	// Configura un manejador de solicitud para la ruta "/json".
	manageServer()

	// Función que verifica la cola de especificaciones constantemente.
	go checkMaquinasVirtualesQueueChanges()

	// Función que verifica la cola de cuentas constantemente.
	go checkManagementQueueChanges()

	// Inicia el servidor HTTP en el puerto 8081.
	fmt.Println("Servidor escuchando en el puerto 8081...")
	if err := http.ListenAndServe(":8081", nil); err != nil {
		fmt.Println("Error al iniciar el servidor:", err)
	}

}

// Funciòn que se encarga de realizar la conexiòn a la base de datos

func manageSqlConecction() {
	var err error

	db, err = sql.Open("mysql", "root:root@tcp(172.17.0.2)/uqcloud")
	if err != nil {
		log.Fatal(err)
	}

}

/*
Funciòn que se encarga de configurar los endpoints, realizar las validaciones correspondientes a los JSON que llegan
por solicitudes HTTP. Se encarga tambièn de ingresar las peticiones para gestiòn de MV a la cola.
Si la peticiòn es de inicio de sesiòn, la gestiona inmediatamente.
*/
func manageServer() {
	maquina_virtualesQueue.Queue = list.New()
	managementQueue.Queue = list.New()

	//Endpoint para las peticiones de creaciòn de màquinas virtuales
	http.HandleFunc("/json/createVirtualMachine", func(w http.ResponseWriter, r *http.Request) {
		// Verifica que la solicitud sea del método POST.
		if r.Method != http.MethodPost {
			http.Error(w, "Se requiere una solicitud POST", http.StatusMethodNotAllowed)
			return
		}

		// Decodifica el JSON recibido en la solicitud en una estructura Specifications.
		var maquina_virtual Maquina_virtual
		decoder := json.NewDecoder(r.Body)
		if err := decoder.Decode(&maquina_virtual); err != nil {
			http.Error(w, "Error al decodificar JSON de especificaciones", http.StatusBadRequest)
			return
		}

		fmt.Println(maquina_virtual)

		// Encola las especificaciones.
		mu.Lock()
		maquina_virtualesQueue.Queue.PushBack(maquina_virtual)
		mu.Unlock()

		fmt.Println("Cantidad de Solicitudes de Especificaciones en la Cola: " + strconv.Itoa(maquina_virtualesQueue.Queue.Len()))

		// Envía una respuesta al cliente.
		response := map[string]string{"mensaje": "Mensaje JSON de especificaciones recibido correctamente"}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	})

	//Endpoint para peticiones de inicio de sesiòn
	http.HandleFunc("/json/login", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Se requiere una solicitud POST", http.StatusMethodNotAllowed)
			return
		}

		var persona Persona
		decoder := json.NewDecoder(r.Body)
		if err := decoder.Decode(&persona); err != nil {
			http.Error(w, "Error al decodificar JSON de inicio de sesión", http.StatusBadRequest)
			return
		}

		// Si las credenciales son válidas, devuelve un JSON con "loginCorrecto" en true, de lo contrario, en false.
		query := "SELECT contrasenia FROM persona WHERE email = ?"
		var resultPassword string

		//Consulta en la base de datos si el usuario existe
		err := db.QueryRow(query, persona.Email).Scan(&resultPassword)

		err2 := bcrypt.CompareHashAndPassword([]byte(resultPassword), []byte(persona.Contrasenia))
		if err2 != nil {
			fmt.Println("Contraseña incorrecta")
		} else {
			fmt.Println("Contraseña correcta")
		}
		if err2 != nil {
			response := map[string]bool{"loginCorrecto": false}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(response)
		} else if err != nil {
			panic(err.Error())
		} else {
			response := map[string]bool{"loginCorrecto": true}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(response)
		}
	})

	//Endpoint para peticiones de inicio de sesiòn
	http.HandleFunc("/json/signin", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Se requiere una solicitud POST", http.StatusMethodNotAllowed)
			return
		}

		var persona Persona
		decoder := json.NewDecoder(r.Body)
		if err := decoder.Decode(&persona); err != nil {
			http.Error(w, "Error al decodificar JSON de inicio de sesión", http.StatusBadRequest)
			return
		}

		printAccount(persona)

		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(persona.Contrasenia), bcrypt.DefaultCost)
		if err != nil {
			fmt.Println("Error al encriptar la contraseña:", err)
			return
		}

		query := "INSERT INTO persona (nombre, apellido, email, contrasenia) VALUES ( ?, ?, ?, ? );"
		var resultUsername string

		//Consulta en la base de datos si el usuario existe
		a, err := db.Exec(query, persona.Nombre, persona.Apellido, persona.Email, hashedPassword)
		fmt.Println(a)
		if err != nil {
			fmt.Println("Error al registrar.")
			response := map[string]bool{"loginCorrecto": false}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(response)
		} else if err != nil {
			panic(err.Error())
		} else {
			fmt.Printf("Registro correcto: %s\n", resultUsername)
			response := map[string]bool{"loginCorrecto": true}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(response)
		}
	})

	//Endpoint para peticiones de inicio de sesiòn
	http.HandleFunc("/json/consultMachine", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Se requiere una solicitud POST", http.StatusMethodNotAllowed)
			return
		}

		var persona Persona
		decoder := json.NewDecoder(r.Body)
		if err := decoder.Decode(&persona); err != nil {
			http.Error(w, "Error al decodificar JSON de inicio de sesión", http.StatusBadRequest)
			return
		}

		query := "SELECT nombre, sistema_operativo, memoria, ip, estado FROM maquina_virtual WHERE persona_email = ?"
		rows, err := db.Query(query, persona.Email)
		if err != nil {
			// Manejar el error
			return
		}
		defer rows.Close()

		var machines []Maquina_virtual
		for rows.Next() {
			var machine Maquina_virtual
			if err := rows.Scan(&machine.Nombre, &machine.Sistema_operativo, &machine.Memoria, &machine.Ip, &machine.Estado); err != nil {
				// Manejar el error al escanear la fila
				continue
			}
			machines = append(machines, machine)
		}
		fmt.Println(machines)

		if err := rows.Err(); err != nil {
			// Manejar el error al iterar a través de las filas
			fmt.Println("no hay nada")
			return
		}

		if len(machines) == 0 {
			// No se encontraron máquinas virtuales para el usuario
			fmt.Println("no hay nada")
			return
		}

		// Respondemos con la lista de máquinas virtuales en formato JSON
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(machines)

	})

	//End point para modificar màquinas virtuales
	http.HandleFunc("/json/modifyVM", func(w http.ResponseWriter, r *http.Request) {
		// Verifica que la solicitud sea del método POST.
		if r.Method != http.MethodPost {
			http.Error(w, "Se requiere una solicitud POST", http.StatusMethodNotAllowed)
			return
		}

		// Decodifica el JSON recibido en la solicitud en un mapa genérico.
		var payload map[string]interface{}
		decoder := json.NewDecoder(r.Body)
		if err := decoder.Decode(&payload); err != nil {
			http.Error(w, "Error al decodificar JSON de la solicitud", http.StatusBadRequest)
			return
		}

		// Verifica que el campo "tipo_solicitud" esté presente y sea "modify".
		tipoSolicitud, isPresent := payload["tipo_solicitud"].(string)
		if !isPresent || tipoSolicitud != "modify" {
			http.Error(w, "El campo 'tipo_solicitud' debe ser 'modify'", http.StatusBadRequest)
			return
		}

		// Extrae el objeto "specifications" del JSON.
		specificationsData, isPresent := payload["specifications"].(map[string]interface{})
		if !isPresent || specificationsData == nil {
			http.Error(w, "El campo 'specifications' es inválido", http.StatusBadRequest)
			return
		}

		// Encola las peticiones.
		mu.Lock()
		managementQueue.Queue.PushBack(payload)
		mu.Unlock()

		// Envía una respuesta al cliente.
		response := map[string]string{"mensaje": "Mensaje JSON de especificaciones para modificar MV recibido correctamente"}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	})

	//End point para eliminar màquinas virtuales
	http.HandleFunc("/json/deleteVM", func(w http.ResponseWriter, r *http.Request) {
		// Verifica que la solicitud sea del método POST.
		if r.Method != http.MethodPost {
			http.Error(w, "Se requiere una solicitud POST", http.StatusMethodNotAllowed)
			return
		}

		var datos map[string]interface{}

		decoder := json.NewDecoder(r.Body)
		if err := decoder.Decode(&datos); err != nil {
			http.Error(w, "Error al decodificar JSON de especificaciones", http.StatusBadRequest)
			return
		}

		// Verificar si el nombre de la máquina virtual, la IP del host y el tipo de solicitud están presentes y no son nulos
		nombre, nombrePresente := datos["nombreVM"].(string)
		tipoSolicitud, tipoPresente := datos["tipo_solicitud"].(string)

		if !tipoPresente || tipoSolicitud != "delete" {
			http.Error(w, "El campo 'tipo_solicitud' debe ser 'delete'", http.StatusBadRequest)
			return
		}

		if !nombrePresente || !tipoPresente || nombre == "" || tipoSolicitud == "" {
			http.Error(w, "El tipo de solicitud y el nombre de la máquina virtual son obligatorios", http.StatusBadRequest)
			return
		}

		// Encola las peticiones.
		mu.Lock()
		managementQueue.Queue.PushBack(datos)
		mu.Unlock()

		// Envía una respuesta al cliente.
		response := map[string]string{"mensaje": "Mensaje JSON para eliminar MV recibido correctamente"}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)

	})

	//------------------------------------------------------------------------------------------------------------
	//End point para encender màquinas virtuales
	http.HandleFunc("/json/startVM", func(w http.ResponseWriter, r *http.Request) {
		// Verifica que la solicitud sea del método POST.
		if r.Method != http.MethodPost {
			http.Error(w, "Se requiere una solicitud POST", http.StatusMethodNotAllowed)
			return
		}

		var datos map[string]interface{}

		decoder := json.NewDecoder(r.Body)
		if err := decoder.Decode(&datos); err != nil {
			http.Error(w, "Error al decodificar JSON de especificaciones", http.StatusBadRequest)
			return
		}

		// Verificar si el nombre de la máquina virtual, la IP del host y el tipo de solicitud están presentes y no son nulos
		nombreVM, nombrePresente := datos["nombreVM"].(string)

		tipoSolicitud, tipoPresente := datos["tipo_solicitud"].(string)

		if !tipoPresente || tipoSolicitud != "start" {
			http.Error(w, "El campo 'tipo_solicitud' debe ser 'start'", http.StatusBadRequest)
			return
		}

		if !nombrePresente || !tipoPresente || nombreVM == "" || tipoSolicitud == "" {
			http.Error(w, "El tipo de solicitud y nombre de la máquina virtual son obligatorios", http.StatusBadRequest)
			return
		}

		// Encola las peticiones.
		mu.Lock()
		managementQueue.Queue.PushBack(datos)
		mu.Unlock()

		// Envía una respuesta al cliente.
		response := map[string]string{"mensaje": "Mensaje JSON para encender MV recibido correctamente"}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)

	})

	//End point para apagar màquinas virtuales
	http.HandleFunc("/json/stopVM", func(w http.ResponseWriter, r *http.Request) {
		// Verifica que la solicitud sea del método POST.
		if r.Method != http.MethodPost {
			http.Error(w, "Se requiere una solicitud POST", http.StatusMethodNotAllowed)
			return
		}

		var datos map[string]interface{}

		decoder := json.NewDecoder(r.Body)
		if err := decoder.Decode(&datos); err != nil {
			http.Error(w, "Error al decodificar JSON de especificaciones", http.StatusBadRequest)
			return
		}

		// Verificar si el nombre de la máquina virtual, la IP del host y el tipo de solicitud están presentes y no son nulos
		nombreVM, nombrePresente := datos["nombreVM"].(string)

		tipoSolicitud, tipoPresente := datos["tipo_solicitud"].(string)

		if !tipoPresente || tipoSolicitud != "stop" {
			http.Error(w, "El campo 'tipo_solicitud' debe ser 'stop'", http.StatusBadRequest)
			return
		}

		if !nombrePresente || !tipoPresente || nombreVM == "" || tipoSolicitud == "" {
			http.Error(w, "El tipo de solicitud y nombre de la máquina virtual son obligatorios", http.StatusBadRequest)
			return
		}

		// Encola las peticiones.
		mu.Lock()
		managementQueue.Queue.PushBack(datos)
		mu.Unlock()

		// Envía una respuesta al cliente.
		response := map[string]string{"mensaje": "Mensaje JSON para apagar MV recibido correctamente"}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)

	})

}

func checkMaquinasVirtualesQueueChanges() {
	for {
		// Verifica si el tamaño de la cola de especificaciones ha cambiado.
		mu.Lock()
		currentQueueSize := maquina_virtualesQueue.Queue.Len()
		mu.Unlock()

		if currentQueueSize > 0 {
			// Imprime y elimina el primer elemento de la cola de especificaciones.
			mu.Lock()
			firstElement := maquina_virtualesQueue.Queue.Front()
			maquina_virtualesQueue.Queue.Remove(firstElement)
			mu.Unlock()

			crateVM(firstElement.Value.(Maquina_virtual)) //En el segundo argumento debe ir la ip del host en el cual se va a crear la VM

			printMaquinaVirtual(firstElement.Value.(Maquina_virtual), true)
		}

		// Espera un segundo antes de verificar nuevamente.
		time.Sleep(1 * time.Second)
	}
}

/*
	Funciòn que se encarga de imprimir peticiones

@specs Este paràmetro contiene las especificaciones de la màquina virtual gestionada
@isCreateVM Variable de tipo booleana que si es verdadera significa que la peticiòn es de crear una màquina virtual. En caso contrario
indica que la peticiòn es para modificar una màquina virtual
*/
func printSpecifications(specs Maquina_virtual, isCreateVM bool) {
	// Crea el comando en VirtualBox
	//comandCreate := "Vboxmanage createvm --name " + specs.Name + " --ostype " + specs.OSType
	//comandModify := "Vboxmanage modifyvm " + specs.Name + " --memory " + strconv.Itoa(specs.Memory) + " --vram 128"

	// Imprime las especificaciones recibidas.
	fmt.Printf("-------------------------\n")
	fmt.Printf("Nombre de la Máquina: %s\n", specs.Nombre)
	fmt.Printf("Sistema Operativo: %s\n", specs.Sistema_operativo)
	fmt.Printf("Memoria Requerida: %d Mb\n", specs.Memoria)
	fmt.Printf("CPU Requerida: %d núcleos\n", specs.Cpu)

}

func printMaquinaVirtual(specs Maquina_virtual, isCreateVM bool) {
	// Crea el comando en VirtualBox
	//comandCreate := "Vboxmanage createvm --name " + specs.Name + " --ostype " + specs.OSType
	//comandModify := "Vboxmanage modifyvm " + specs.Name + " --memory " + strconv.Itoa(specs.Memory) + " --vram 128"

	// Imprime las especificaciones recibidas.
	fmt.Printf("-------------------------\n")
	fmt.Printf("Nombre de la Máquina: %s\n", specs.Nombre)
	fmt.Printf("Sistema Operativo: %s\n", specs.Sistema_operativo)
	fmt.Printf("Memoria Requerida: %d Mb\n", specs.Memoria)
	fmt.Printf("CPU Requerida: %d núcleos\n", specs.Cpu)

}

func printAccount(account Persona) {
	// Imprime la cuenta recibida.
	fmt.Printf("-------------------------\n")
	fmt.Printf("Nombre de Usuario: %s\n", account.Nombre)
	fmt.Printf("Contraseña: %s\n", account.Contrasenia)
	fmt.Printf("Email: %s\n", account.Email)

}

/*
Esta funciòn carga y devuelve la llave privada SSH desde la ruta especificada
@file Paràmetro que contiene la ruta de la llave privada
*/
func privateKeyFile(file string) (ssh.AuthMethod, error) {
	buffer, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}

	key, err := ssh.ParsePrivateKey(buffer)
	if err != nil {
		return nil, err
	}

	return ssh.PublicKeys(key), nil
}

/*
Funciòn que se encarga de realizar la configuraciòn SSH con el host
@user Paràmetro que contiene el nombre del usuario al cual se va a conectar
@privateKeyPath Paràmetro que contiene la ruta de la llave privada SSH
*/
func configurarSSH(user string, privateKeyPath string) (*ssh.ClientConfig, error) {
	authMethod, err := privateKeyFile(privateKeyPath)
	if err != nil {
		return nil, err
	}

	config := &ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{
			authMethod,
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}
	return config, nil
}

/*
	Funciòn que se encarga de enviar los comandos a travès de la conexiòn SSH con el host

@host Paràmetro que contien la direcciòn IP del host al cual le va a enviar los comandos
@comando Paràmetro que contiene la instrucciòn que se desea ejecutar en el host
@config Paràmetro que contiene la configuraciòn SSH
@return Retorna la respuesta del host si la hay
*/
func enviarComandoSSH(host string, comando string, config *ssh.ClientConfig) (salida string) {

	//Establece la conexiòn SSH
	conn, err := ssh.Dial("tcp", host+":22", config)
	if err != nil {
		log.Fatalf("Error al establecer la conexiòn SSH: %s", err)
	}
	defer conn.Close()

	//Crea una nueva sesiòn SSH
	session, err := conn.NewSession()
	if err != nil {
		log.Fatalf("Error al crear la sesiòn SSH: %s", err)
	}
	defer session.Close()

	//Ejecuta el comando remoto
	output, err := session.CombinedOutput(comando)
	if err != nil {
		fmt.Println(err)
		fmt.Println(comando)
		log.Fatalf("Error al ejecutar el comando remoto: %s", err)
	}

	//Imprime la salida del comado
	fmt.Println(string(output))

	return string(output)
}

/*
	Esta funciòn permite enviar los comandos VBoxManage necesarios para crear una nueva màquina virtual

@spects Paràmetro que contiene la configuraciòn enviarda por el usuario para crear la MV
@hostIP Paràmetro que contiene la direcciòn IP del host--------------------------------------
@config Paràmetro que contiene la configuraciòn de la conexiòn SSH con el host
*/
func crateVM(specs Maquina_virtual) {

	//Selecciona un host al azar
	host := selectHost()

	// Procesa el primer elemento (en este caso, imprime las especificaciones).
	config, err := configurarSSH(host.Hostname, *privateKeyPath)
	if err != nil {
		log.Fatal("Error al configurar SSH:", err)
		return
	}

	//Comando para crear una màquina virtual
	createVM := "VBoxManage createvm --name " + specs.Nombre + " --ostype " + specs.Sistema_operativo + " --register"
	uuid := enviarComandoSSH(host.Ip, createVM, config)

	//Comando para asignar la memoria RAM a la MV
	memoryCommand := "VBoxManage modifyvm " + specs.Nombre + " --memory " + strconv.Itoa(specs.Memoria)
	enviarComandoSSH(host.Ip, memoryCommand, config)

	//Comando para agregar el controlador de almacenamiento
	sctlCommand := "VBoxManage storagectl " + specs.Nombre + " --name hardisk --add sata"
	enviarComandoSSH(host.Ip, sctlCommand, config)

	//Comando para conectar el disco multiconexiòn a la MV
	sattachCommand := "VBoxManage storageattach " + specs.Nombre + " --storagectl hardisk --port 0 --device 0 --type hdd --medium " + "\"" + host.Ruta_disco_multi + "\""
	enviarComandoSSH(host.Ip, sattachCommand, config)

	//Comando para asignar las unidades de procesamiento
	cpuCommand := "VBoxManage modifyvm " + specs.Nombre + " --cpus " + strconv.Itoa(specs.Cpu)
	enviarComandoSSH(host.Ip, cpuCommand, config)

	//comando para poner el adaptador de red en modo puente (Bridge)
	redAdapterCommand := "VBoxManage modifyvm " + specs.Nombre + " --nic1 bridged --bridgeadapter1 " + "\"" + host.Adaptador_red + "\""
	enviarComandoSSH(host.Ip, redAdapterCommand, config)

	lines := strings.Split(string(uuid), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "UUID:") {
			uuid = strings.TrimPrefix(line, "UUID:")
		}
	}

	nuevaMaquinaVirtual := Maquina_virtual{
		Uuid:              uuid,
		Nombre:            specs.Nombre,
		Sistema_operativo: specs.Sistema_operativo,
		Memoria:           specs.Memoria,
		Cpu:               specs.Cpu,
		Estado:            "Apagado",
		Hostname:          "uqcloud",
		Persona_email:     specs.Persona_email,
	}

	fmt.Println(specs)

	_, err1 := db.Exec("INSERT INTO maquina_virtual (uuid, nombre, sistema_operativo, memoria, cpu, estado,persona_email, host_id, hostname, ip) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		nuevaMaquinaVirtual.Uuid, nuevaMaquinaVirtual.Nombre, nuevaMaquinaVirtual.Sistema_operativo, nuevaMaquinaVirtual.Memoria,
		nuevaMaquinaVirtual.Cpu, nuevaMaquinaVirtual.Estado, nuevaMaquinaVirtual.Persona_email, host.Id, nuevaMaquinaVirtual.Hostname, nuevaMaquinaVirtual.Ip)

	if err1 != nil {
		log.Fatal(err)
	}

	fmt.Println("Màquina virtual creada con èxito")
}

/*
	Esta funciòn verifica si una màquina virtual està encendida

@nameVM Paràmetro que contiene el nombre de la màquina virtual a verificar
@hostIP Paràmetro que contiene la direcciòn Ip del host en el cual està la MV
@return Retorna true si la màquina està encendida o false en caso contrario
*/
func isRunning(nameVM string, hostIP string, config *ssh.ClientConfig) (running bool) {

	//Comando para saber el estado de una màquina virtual
	command := "VBoxManage showvminfo " + nameVM + " | findstr /C:\"State:\""
	running = false

	salida := enviarComandoSSH(hostIP, command, config)

	// Expresión regular para buscar el estado (running)
	regex := regexp.MustCompile(`State:\s+(running|powered off)`)
	matches := regex.FindStringSubmatch(salida)

	// matches[1] contendrá "running" o "powered off" dependiendo del estado
	if len(matches) > 1 {
		estado := matches[1]
		fmt.Println("Estado:", estado)
		if estado == "running" {
			running = true
		}
	}
	return running
}

/* Funciòn que contiene los comandos necesarios para modificar una màquina virtual. Primero verifica
si la màquina esta encendida o apagada. En caso de que estè encendida, invoca la funciòn para apagar la màquina.
@specs Paràmetro que contiene las especificaciones a modificar en la màquina virtual
@hostIP Paràmetro que contiene la direcciòn ip del host en el cual està alojada la màquina virtual a modificar
*/

func modifyVM(specs Maquina_virtual, hostIP string, config *ssh.ClientConfig) string {

	//Comando para modificar la memoria RAM a la MV
	memoryCommand := "VBoxManage modifyvm " + specs.Nombre + " --memory " + strconv.Itoa(specs.Memoria)

	//Comando para modificar las unidades de procesamiento
	cpuCommand := "VBoxManage modifyvm " + specs.Nombre + " --cpus " + strconv.Itoa(specs.Cpu)

	//Variable que contiene el estado de la MV (Encendida o apagada)
	running := isRunning(specs.Nombre, hostIP, config)

	if running {
		fmt.Println("Para modificar la màquina primero debe apagarla")
		return "Para modificar la màquina primero debe apagarla"
	}

	if specs.Cpu != 0 {
		enviarComandoSSH(hostIP, cpuCommand, config)
	}

	if specs.Memoria != 0 {
		enviarComandoSSH(hostIP, memoryCommand, config)
	}

	fmt.Println("Las modificaciones se realizaron correctamente")
	return "Modificaciones realizadas con èxito"
}

/* Funciòn que permite enviar los comandos necesarios para apagar una màquina virtual
@nameVM Paràmetro que contiene el nombre de la màquina virtual a apagar
@hostIP Paràmetro que contiene la direcciòn ip del host
*/

func apagarMV(nameVM string) {

	//Obtiene el objeto "maquina_virtual"
	maquinaVirtual := getVM(nameVM)

	//Obtiene el host en el cual està alojada la MV
	host := getHost(maquinaVirtual.Host_id)

	config, err := configurarSSH(host.Hostname, *privateKeyPath)
	if err != nil {
		fmt.Println("Error al configurar SSH:", err)
	}

	//Variable que contiene el estado de la MV (Encendida o apagada)
	running := isRunning(nameVM, host.Ip, config)

	if !running {

		startVM(nameVM)
	} else {
		//Comando para enviar señal de apagado a la MV esperando que los programas cierren correctamente
		acpiCommand := "VBoxManage controlvm " + nameVM + " acpipowerbutton"

		//Comando para apagar la màquina sin esperar que los programas cierren
		powerOffCommand := "VBoxManage controlvm " + nameVM + " poweroff"

		fmt.Println("Apagando màquina " + nameVM + "...")

		enviarComandoSSH(host.Ip, acpiCommand, config)

		// Establece un temporizador de espera máximo de 5 minutos
		maxEspera := time.Now().Add(5 * time.Minute)

		// Espera hasta que la máquina esté apagada o haya pasado el tiempo máximo de espera
		for time.Now().Before(maxEspera) {
			if !isRunning(nameVM, host.Ip, config) {
				break
			}

			// Espera un 5 segundos antes de volver a verificar el estado de la màquina
			time.Sleep(5 * time.Second)
		}

		if isRunning(nameVM, host.Ip, config) {
			enviarComandoSSH(host.Ip, powerOffCommand, config)
		}
		_, err1 := db.Exec("UPDATE maquina_virtual set estado = 'Apagado' WHERE NOMBRE = ?", nameVM)
		if err1 != nil {
			fmt.Println("Error al realizar la actualizaciòn del estado", err1)
		}

		fmt.Println("Màquina apagada con èxito")
	}

}

/*Funciòn que se encarga de gestionar la cola de solicitudes para la gestiòn de màquinas virtuales
 */
func checkManagementQueueChanges() {
	for {
		// Verifica si el tamaño de la cola de especificaciones ha cambiado.
		mu.Lock()
		currentQueueSize := managementQueue.Queue.Len()
		mu.Unlock()

		if currentQueueSize > 0 {
			// Imprime y elimina el primer elemento de la cola de especificaciones.
			mu.Lock()
			firstElement := managementQueue.Queue.Front()

			// Verifica que el primer elemento sea un mapa.
			data, dataPresent := firstElement.Value.(map[string]interface{})

			if !dataPresent {
				fmt.Println("No se pudo procesar la solicitud")
				mu.Unlock()
				return
			}

			config, err := configurarSSH("jhoiner", *privateKeyPath)
			if err != nil {
				log.Fatal("Error al configurar SSH:", err)
				return
			}

			// Obtiene el valor del campo "tipo_solicitud"
			tipoSolicitud, _ := data["tipo_solicitud"].(string)

			if strings.ToLower(tipoSolicitud) == "modify" {
				specsMap, _ := data["specifications"].(map[string]interface{})

				// Convierte el mapa de especificaciones a un objeto Specifications.
				specsJSON, err := json.Marshal(specsMap)
				if err != nil {
					fmt.Println("Error al serializar las especificaciones:", err)
					mu.Unlock()
					return
				}

				var specifications Maquina_virtual
				err = json.Unmarshal(specsJSON, &specifications)
				if err != nil {
					fmt.Println("Error al deserializar las especificaciones:", err)
					mu.Unlock()
					return
				}

				modifyVM(specifications, "192.168.101.10", config)
			}

			//--------------------------------------------------------------------------------------
			if strings.ToLower(tipoSolicitud) == "delete" {

				// Obtiene el valor del campo "tipo_solicitud"
				nameVM, _ := data["nombreVM"].(string)

				deleteVM(nameVM)
			}

			if strings.ToLower(tipoSolicitud) == "start" {

				// Obtiene el valor del campo "tipo_solicitud"
				nameVM, _ := data["nombreVM"].(string)

				startVM(nameVM)
			}

			if strings.ToLower(tipoSolicitud) == "stop" {

				// Obtiene el valor del campo "tipo_solicitud"
				nameVM, _ := data["nombreVM"].(string)

				apagarMV(nameVM)
			}

			managementQueue.Queue.Remove(firstElement)
			mu.Unlock()
		}

		// Espera un segundo antes de verificar nuevamente.
		time.Sleep(1 * time.Second)
	}
}

/* Funciòn que permite enviar los comandos necesarios para eliminar una màquina virtual
@nameVM Paràmetro que contiene el nombre de la màquina virtual a eliminar
*/

func deleteVM(nameVM string) string {

	//Obtiene el objeto "maquina_virtual"
	maquinaVirtual := getVM(nameVM)

	//Obtiene el host en el cual està alojada la MV
	host := getHost(maquinaVirtual.Host_id)

	config, err := configurarSSH(host.Hostname, *privateKeyPath)
	if err != nil {
		fmt.Println("Error al configurar SSH:", err)
	}

	//Comando para desconectar el disco de la MV
	disconnectCommand := "VBoxManage storageattach " + nameVM + " --storagectl hardisk --port 0 --device 0 --medium none"

	//Comando para eliminar la MV
	deleteCommand := "VBoxManage unregistervm " + nameVM + " --delete"

	//Variable que contiene el estado de la MV (Encendida o apagada)
	running := isRunning(nameVM, host.Ip, config)

	if running {
		fmt.Println("Debe apagar la màquina para eliminarla")
		return "Debe apagar la màquina para eliminarla"

	} else {
		enviarComandoSSH(host.Ip, disconnectCommand, config)
		enviarComandoSSH(host.Ip, deleteCommand, config)

		err := db.QueryRow("DELETE FROM maquina_virtual WHERE NOMBRE = ?", nameVM)
		if err == nil {
			fmt.Println("Error al eliminar el registro de la base de datos: ", err)
		}
	}

	fmt.Println("Màquina eliminada correctamente")
	return "Màquina eliminada correctamente"
}

/*
Funciòn que permite iniciar una màquina virtual en modo "headless", lo que indica que se inicia en segundo plano
para que el usuario de la màquina fìsica no se vea afectado
@specs Contiene las especificaciones de la maquina a encender
@hostIp Contiene la direcciòn Ip del host en el cual està alojada la MV
@return Retorna la direcciòn Ip de la màquina virtual
*/

func startVM(nameVM string) string {

	//Obtiene el objeto "maquina_virtual"
	maquinaVirtual := getVM(nameVM)

	//Obtiene el host en el cual està alojada la MV
	host := getHost(maquinaVirtual.Host_id)

	config, err := configurarSSH(host.Hostname, *privateKeyPath)
	if err != nil {
		fmt.Println("Error al configurar SSH:", err)
	}

	//Variable que contiene el estado de la MV (Encendida o apagada)
	running := isRunning(nameVM, host.Ip, config)

	if running {

		apagarMV(nameVM)
		return ""
	} else {
		fmt.Println("Encendiendo la màquina " + nameVM + "...")

		// Comando para encender la máquina virtual
		startVMCommand := "VBoxManage startvm " + nameVM + " --type headless"
		enviarComandoSSH(host.Ip, startVMCommand, config)

		fmt.Println("Obteniendo direcciòn IP...")
		// Espera 10 segundos para que la máquina virtual inicie
		time.Sleep(10 * time.Second)

		// Obtiene la dirección IP de la máquina virtual después de que se inicie
		getIpCommand := "VBoxManage guestproperty get " + nameVM + " /VirtualBox/GuestInfo/Net/0/V4/IP"
		var ipAddress string

		for ipAddress == "" || ipAddress == "No value set!" {
			ipAddress = strings.TrimSpace(enviarComandoSSH(host.Ip, getIpCommand, config))
			if ipAddress == "No value set!" {
				time.Sleep(5 * time.Second) // Espera 5 segundos antes de intentar nuevamente
				fmt.Println("Obteniendo direcciòn IP...")
			}

		}

		//Almacena solo el valor de la IP quitàndole el texto "Value:"
		ipAddress = strings.TrimPrefix(ipAddress, "Value:")
		ipAddress = strings.TrimSpace(ipAddress)

		_, err1 := db.Exec("UPDATE maquina_virtual set estado = 'Encendido' WHERE NOMBRE = ?", nameVM)
		if err1 != nil {
			fmt.Println("Error al realizar la actualizaciòn del estado", err1)
		}
		_, err2 := db.Exec("UPDATE maquina_virtual set ip = ? WHERE NOMBRE = ?", ipAddress, nameVM)
		if err2 != nil {
			fmt.Println("Error al realizar la actualizaciòn de la ip", err2)
		}
		fmt.Println("Màquina encendida, la direcciòn IP es: " + ipAddress)

		return ipAddress
	}

}

/*
Funciòn que contiene el algoritmo de asignaciòn tipo aleatorio. Se encarga de escoger un host de la base de datos al azar
Return Retorna el host seleccionado.
*/

func selectHost() Host {

	// Consulta para contar el número de registros en la tabla "host"
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM host").Scan(&count)
	if err != nil {
		log.Fatal(err)
	}

	// Genera un número aleatorio dentro del rango de registros
	rand.New(rand.NewSource(time.Now().Unix())) // Seed para generar números aleatorios diferentes en cada ejecución
	randomIndex := rand.Intn(count)

	// Consulta para seleccionar un registro aleatorio de la tabla "host"
	var host Host

	err = db.QueryRow("SELECT * FROM host LIMIT ?, 1", randomIndex).Scan(&host.Id, &host.Nombre, &host.Mac, &host.Memoria, &host.Cpu, &host.Adaptador_red, &host.Almacenamiento_total, &host.Estado, &host.Sistema_operativo, &host.Ruta_disco_multi, &host.Ruta_llave_ssh, &host.Hostname, &host.Ip)
	if err != nil {
		log.Fatal(err)
	}

	// Imprime el registro aleatorio seleccionado
	fmt.Printf("Registro aleatorio seleccionado:\n")
	fmt.Printf("ID: %d, Nombre: %s, MAC: %s, Memoria: %d, CPU: %d, Adaptador Red: %s, Estado: %s, SO: %s, Ruta Disco Multi: %s, Ruta Llave SSH: %s, Hostname: %s, IP: %s\n", host.Id, host.Nombre, host.Mac, host.Memoria, host.Cpu, host.Adaptador_red, host.Estado, host.Sistema_operativo, host.Ruta_disco_multi, host.Ruta_llave_ssh, host.Hostname, host.Ip)

	return host
}

func existVM(nameVM string) bool {

	var existe bool
	err := db.QueryRow("SELECT EXISTS(SELECT 1 FROM maquinas_virtuales WHERE nombre = ?)", nameVM).Scan(&existe)
	if err != nil {
		log.Fatal(err)
	}

	if existe {
		return true
	}

	return false
}

func getHost(idHost string) Host {

	var host Host
	err := db.QueryRow("SELECT * FROM host WHERE id = ?", idHost).Scan(&host.Id, &host.Nombre, &host.Mac, &host.Memoria, &host.Cpu, &host.Adaptador_red, &host.Almacenamiento_total, &host.Estado, &host.Sistema_operativo, &host.Ruta_disco_multi, &host.Ruta_llave_ssh, &host.Hostname, &host.Ip)
	if err != nil {
		if err == sql.ErrNoRows {
			fmt.Println("No se encontró el host con el nombre especificado.")
		} else {
			log.Fatal(err)
		}
	}

	return host

}

func getVM(nameVM string) Maquina_virtual {

	var maquinaVirtual Maquina_virtual
	err := db.QueryRow("SELECT * FROM maquina_virtual WHERE nombre = ?", nameVM).Scan(&maquinaVirtual.Uuid, &maquinaVirtual.Nombre, &maquinaVirtual.Sistema_operativo, &maquinaVirtual.Memoria, &maquinaVirtual.Cpu, &maquinaVirtual.Estado, &maquinaVirtual.Persona_email, &maquinaVirtual.Host_id, &maquinaVirtual.Hostname, &maquinaVirtual.Ip)
	if err != nil {
		if err == sql.ErrNoRows {
			fmt.Println("No se encontró la màquina virtual con el nombre especificado.")
		} else {
			log.Fatal(err)
		}
	}

	return maquinaVirtual
}

func getUser(email string) Persona {

	var persona Persona
	err := db.QueryRow("SELECT * FROM persona WHERE email = ?", email).Scan(&persona.Email, &persona.Nombre, &persona.Apellido, &persona.Contrasenia)
	if err != nil {
		if err == sql.ErrNoRows {
			fmt.Println("No se encontrò un usuario con el email especificado")
		} else {
			log.Fatal(err)
		}
	}

	return persona

}
