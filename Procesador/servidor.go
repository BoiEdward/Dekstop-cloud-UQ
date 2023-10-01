package main

import (
	"container/list"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"

	_ "github.com/go-sql-driver/mysql"
)

var db *sql.DB

// Variable que almacena la ruta de la llave privada ingresada por paametro cuando de ejecuta el programa
var privateKeyPath = flag.String("key", "", "Ruta de la llave privada SSH")

/*
Estructura tipo JSON que contiene los datos que ingresa el usuario para la creaciòn de una MV
@Name Este campo representa el nombre de la màquina virtual a crear
@OSType Este campo representa el tipo de sistema operativo de la màquina virtual
@Memory Este campo representa la cantidad de memoria RAM a asiganar a la MV
@CPU Este campo representa la cantidad de unidades de procesamiento de la MV
*/
type Specifications struct {
	Name   string `json:"nombre"`
	OSType string `json:"tipoSO"`
	Memory int    `json:"memoria"`
	CPU    int    `json:"cpu"`
}

/*
Estrucutura de datos tipo JSON que contiene los campos necesarios para iniciar sesiòn
@Username Este campo representa el nombre de usuario
@Password Este campo representa la contraseña
*/
type Account struct {
	Username string `json:"nombre"`
	Password string `json:"contrasenia"`
}

// Cola de especificaciones para la creaciòn de màquinas virtuales
type SpecificationsQueue struct {
	sync.Mutex
	Queue *list.List
}

// Cola de especificaciones para la modificaciòn de màquinas virtuales
type ModifyQueue struct {
	sync.Mutex
	Queue *list.List
}

// Declaraciòn de variables globales
var (
	specificationsQueue SpecificationsQueue
	modifyQueue         ModifyQueue
	mu                  sync.Mutex
	lastQueueSize       int
)

func main() {

	flag.Parse()

	//Verifica que el paràmetro de la ruta de la llave privada no estè vacìo
	if *privateKeyPath == "" {
		fmt.Println("Debe ingresar la ruta de la llave privada SSH")
		return
	}

	// Conexión a SQL
	//manageSqlConecction()

	// Configura un manejador de solicitud para la ruta "/json".
	manageServer()

	// Configurar la conexión SSH

	// Función que verifica la cola de especificaciones constantemente.
	go checkSpecificationsQueueChanges()

	// Función que verifica la cola de cuentas constantemente.
	go checkModifyQueueChanges()

	// Inicia el servidor HTTP en el puerto 8081.
	fmt.Println("Servidor escuchando en el puerto 8081...")
	if err := http.ListenAndServe(":8081", nil); err != nil {
		fmt.Println("Error al iniciar el servidor:", err)
	}

}

// Funciòn que se encarga de realizar la conexiòn a la base de datos

func manageSqlConecction() {
	var err error
	db, err = sql.Open("mysql", "root:root@tcp(172.17.0.2:3306)/decktop_cloud_data_base")
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
	specificationsQueue.Queue = list.New()
	modifyQueue.Queue = list.New()

	//Endpoint para las peticiones de creaciòn de màquinas virtuales
	http.HandleFunc("/json/specifications", func(w http.ResponseWriter, r *http.Request) {
		// Verifica que la solicitud sea del método POST.
		if r.Method != http.MethodPost {
			http.Error(w, "Se requiere una solicitud POST", http.StatusMethodNotAllowed)
			return
		}

		// Decodifica el JSON recibido en la solicitud en una estructura Specifications.
		var specifications Specifications
		decoder := json.NewDecoder(r.Body)
		if err := decoder.Decode(&specifications); err != nil {
			http.Error(w, "Error al decodificar JSON de especificaciones", http.StatusBadRequest)
			return
		}

		// Encola las especificaciones.
		mu.Lock()
		specificationsQueue.Queue.PushBack(specifications)
		mu.Unlock()

		fmt.Println("Cantidad de Solicitudes de Especificaciones en la Cola: " + strconv.Itoa(specificationsQueue.Queue.Len()))

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

		var account Account
		decoder := json.NewDecoder(r.Body)
		if err := decoder.Decode(&account); err != nil {
			http.Error(w, "Error al decodificar JSON de inicio de sesión", http.StatusBadRequest)
			return
		}

		printAccount(account)

		// Si las credenciales son válidas, devuelve un JSON con "loginCorrecto" en true, de lo contrario, en false.
		query := "SELECT username FROM account WHERE username = ? AND password = ?"
		var resultUsername string

		//Consulta en la base de datos si el usuario existe
		err := db.QueryRow(query, account.Username, account.Password).Scan(&resultUsername)
		if err == sql.ErrNoRows {
			fmt.Println("Usuario no encontrado.")
			response := map[string]bool{"loginCorrecto": false}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(response)
		} else if err != nil {
			panic(err.Error())
		} else {
			fmt.Printf("Usuario encontrado: %s\n", resultUsername)
			response := map[string]bool{"loginCorrecto": true}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(response)
		}
	})

	//End point para modificar màquinas virtuales
	http.HandleFunc("/json/modifyVM", func(w http.ResponseWriter, r *http.Request) {
		// Verifica que la solicitud sea del método POST.
		if r.Method != http.MethodPost {
			http.Error(w, "Se requiere una solicitud POST", http.StatusMethodNotAllowed)
			return
		}

		// Decodifica el JSON recibido en la solicitud en una estructura Specifications.
		var specifications Specifications
		decoder := json.NewDecoder(r.Body)
		if err := decoder.Decode(&specifications); err != nil {
			http.Error(w, "Error al decodificar JSON de especificaciones", http.StatusBadRequest)
			return
		}

		// Encola las especificaciones.
		mu.Lock()
		modifyQueue.Queue.PushBack(specifications)
		mu.Unlock()

		// Envía una respuesta al cliente.
		response := map[string]string{"mensaje": "Mensaje JSON de especificaciones para modificar MV recibido correctamente"}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	})

}

/* Funciòn que se encarga de gestionar la cola de peticiones para la creaciòn de màquinas virtuales
 */
func checkSpecificationsQueueChanges() {
	for {
		// Verifica si el tamaño de la cola de especificaciones ha cambiado.
		mu.Lock()
		currentQueueSize := specificationsQueue.Queue.Len()
		mu.Unlock()

		if currentQueueSize > 0 {
			// Imprime y elimina el primer elemento de la cola de especificaciones.
			mu.Lock()
			firstElement := specificationsQueue.Queue.Front()
			specificationsQueue.Queue.Remove(firstElement)
			mu.Unlock()

			// Procesa el primer elemento (en este caso, imprime las especificaciones).
			config, err := configurarSSH("jhoiner", *privateKeyPath)
			if err != nil {
				log.Fatal("Error al configurar SSH:", err)
				return
			}

			crateVM(firstElement.Value.(Specifications), "192.168.101.10", config) //En el segundo argumento debe ir la ip del host en el cual se va a crear la VM

			printSpecifications(firstElement.Value.(Specifications), true)
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
func printSpecifications(specs Specifications, isCreateVM bool) {
	// Crea el comando en VirtualBox
	//comandCreate := "Vboxmanage createvm --name " + specs.Name + " --ostype " + specs.OSType
	//comandModify := "Vboxmanage modifyvm " + specs.Name + " --memory " + strconv.Itoa(specs.Memory) + " --vram 128"

	// Imprime las especificaciones recibidas.
	fmt.Printf("-------------------------\n")
	fmt.Printf("Nombre de la Máquina: %s\n", specs.Name)
	fmt.Printf("Sistema Operativo: %s\n", specs.OSType)
	fmt.Printf("Memoria Requerida: %d Mb\n", specs.Memory)
	fmt.Printf("CPU Requerida: %d núcleos\n", specs.CPU)

}

func printAccount(account Account) {
	// Imprime la cuenta recibida.
	fmt.Printf("-------------------------\n")
	fmt.Printf("Nombre de Usuario: %s\n", account.Username)
	fmt.Printf("Contraseña: %s\n", account.Password)
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
		log.Fatalf("Error al ejecutar el comando remoto: %s", err)
	}

	//Imprime la salida del comado
	fmt.Println(string(output))

	return string(output)
}

/*
	Esta funciòn permite enviar los comandos VBoxManage necesarios para crear una nueva màquina virtual

@spects Paràmetro que contiene la configuraciòn enviarda por el usuario para crear la MV
@hostIP Paràmetro que contiene la direcciòn IP del host
@config Paràmetro que contiene la configuraciòn de la conexiòn SSH con el host
*/
func crateVM(specs Specifications, hostIP string, config *ssh.ClientConfig) {

	//Comando para crear una màquina virtual
	createVM := "VBoxManage createvm --name " + specs.Name + " --ostype " + specs.OSType + " --register"
	enviarComandoSSH(hostIP, createVM, config)

	//Comando para asignar la memoria RAM a la MV
	memoryCommand := "VBoxManage modifyvm " + specs.Name + " --memory " + strconv.Itoa(specs.Memory)
	enviarComandoSSH(hostIP, memoryCommand, config)

	//Comando para agregar el controlador de almacenamiento
	sctlCommand := "VBoxManage storagectl " + specs.Name + " --name hardisk --add sata"
	enviarComandoSSH(hostIP, sctlCommand, config)

	//Comando para conectar el disco multiconexiòn a la MV
	//sattachCommand := "VBoxManage storageattach " + spects.Name + " --storagectl hardisk --port 0 --device 0 --type hdd --medium C:/users/jhoiner/disks/debian.vdi"
	sattachCommand := "VBoxManage storageattach " + specs.Name + " --storagectl hardisk --port 0 --device 0 --type hdd --medium \"C:/users/jhoiner/VirtualBox VMs/Discos/New Debian base.vdi\""
	enviarComandoSSH(hostIP, sattachCommand, config)

	//Comando para asignar las unidades de procesamiento
	cpuCommand := "VBoxManage modifyvm " + specs.Name + " --cpus " + strconv.Itoa(specs.CPU)
	enviarComandoSSH(hostIP, cpuCommand, config)

	//comando para poner el adaptador de red en modo puente (Bridge)
	redAdapterCommand := "VBoxManage modifyvm " + specs.Name + " --nic1 bridged --bridgeadapter1 \"Realtek RTL8723DE 802.11b/g/n PCIe Adapter\""
	enviarComandoSSH(hostIP, redAdapterCommand, config)

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

func modifyVM(specs Specifications, hostIP string, config *ssh.ClientConfig) {

	//Comando para modificar la memoria RAM a la MV
	memoryCommand := "VBoxManage modifyvm " + specs.Name + " --memory " + strconv.Itoa(specs.Memory)

	//Comando para modificar las unidades de procesamiento
	cpuCommand := "VBoxManage modifyvm " + specs.Name + " --cpus " + strconv.Itoa(specs.CPU)

	//Variable que contiene el estado de la MV (Encendida o apagada)
	running := isRunning(specs.Name, hostIP, config)

	if running {
		apagarMV(specs.Name, hostIP, config)

	}

	if specs.CPU != 0 {
		enviarComandoSSH(hostIP, cpuCommand, config)
	}

	if specs.Memory != 0 {
		enviarComandoSSH(hostIP, memoryCommand, config)
	}

	fmt.Println("Las modificaciones se realizaron correctamente")
}

/* Funciòn que permite enviar los comandos necesarios para apagar una màquina virtual
@nameVM Paràmetro que contiene el nombre de la màquina virtual a apagar
@hostIP Paràmetro que contiene la direcciòn ip del host
*/

func apagarMV(nameVM string, hostIP string, config *ssh.ClientConfig) {

	//Comando para enviar señal de apagado a la MV esperando que los programas cierren correctamente
	acpiCommand := "VBoxManage controlvm " + nameVM + " acpipowerbutton"

	//Comando para apagar la màquina sin esperar que los programas cierren
	powerOffCommand := "VBoxManage controlvm " + nameVM + " poweroff"

	enviarComandoSSH(hostIP, acpiCommand, config)

	// Establece un temporizador de espera máximo de 5 minutos
	maxEspera := time.Now().Add(5 * time.Minute)

	// Espera hasta que la máquina esté apagada o haya pasado el tiempo máximo de espera
	for time.Now().Before(maxEspera) {
		if !isRunning(nameVM, hostIP, config) {
			break
		}

		// Espera un 5 segundos antes de volver a verificar el estado de la màquina
		time.Sleep(5 * time.Second)
	}

	if isRunning(nameVM, hostIP, config) {
		enviarComandoSSH(hostIP, powerOffCommand, config)
	}

}

/*Funciòn que se encarga de gestionar la cola de peticiones para la modificaciòn de màquinas virtuales
 */
func checkModifyQueueChanges() {
	for {
		// Verifica si el tamaño de la cola de especificaciones ha cambiado.
		mu.Lock()
		currentQueueSize := modifyQueue.Queue.Len()
		mu.Unlock()

		if currentQueueSize > 0 {
			// Imprime y elimina el primer elemento de la cola de especificaciones.
			mu.Lock()
			firstElement := modifyQueue.Queue.Front()
			modifyQueue.Queue.Remove(firstElement)
			mu.Unlock()

			// Procesa el primer elemento (en este caso, imprime las especificaciones).
			config, err := configurarSSH("jhoiner", *privateKeyPath)
			if err != nil {
				log.Fatal("Error al configurar SSH:", err)
				return
			}

			modifyVM(firstElement.Value.(Specifications), "192.168.101.10", config)

			printSpecifications(firstElement.Value.(Specifications), false)
		}

		// Espera un segundo antes de verificar nuevamente.
		time.Sleep(1 * time.Second)
	}
}
