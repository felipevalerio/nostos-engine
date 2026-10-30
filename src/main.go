package main

import (
	"fmt"
	"runtime"
	"syscall"
	"unsafe"
)

// ----------------------------------------------------------------------------
// DLLs do Windows e funções
// -----------------------------------------------------------------------------

var (
	user32   = syscall.NewLazyDLL("user32.dll")
	kernel32 = syscall.NewLazyDLL("kernel32.dll")
	gdi32    = syscall.NewLazyDLL("gdi32.dll")

	procRegisterClassExW = user32.NewProc("RegisterClassExW")
	procCreateWindowExW  = user32.NewProc("CreateWindowExW")
	procDefWindowProcW   = user32.NewProc("DefWindowProcW")
	procShowWindow       = user32.NewProc("ShowWindow")
	procUpdateWindow     = user32.NewProc("UpdateWindow")
	procGetMessageW      = user32.NewProc("GetMessageW")
	procTranslateMessage = user32.NewProc("TranslateMessage")
	procDispatchMessageW = user32.NewProc("DispatchMessageW")
	procPostQuitMessage  = user32.NewProc("PostQuitMessage")
	procBeginPaint       = user32.NewProc("BeginPaint")
	procEndPaint         = user32.NewProc("EndPaint")
	procFillRect         = user32.NewProc("FillRect")

	procCreateSolidBrush = gdi32.NewProc("CreateSolidBrush")
	procDeleteObject     = gdi32.NewProc("DeleteObject")

	procGetModuleHandleW = kernel32.NewProc("GetModuleHandleW")
)

// ----------------------------------------------------------------------------
// Constantes da WinAPI.
// Definindo os estilos de classe de janela da API Win32
// ----------------------------------------------------------------------------

const (
	CS_HREDRAW = 0x0002
	CS_VREDRAW = 0x0001

	WS_OVERLAPPEDWINDOW = 0x00CF0000

	CW_USEDEFAULT = 0x80000000

	SW_SHOW = 5

	WM_PAINT   = 0x000F // 15
	WM_DESTROY = 0x0002 // 2
	WM_QUIT    = 0x0012 // 18
)

// ----------------------------------------------------------------------------
// Constantes do framebuffer.
// Definindo a largura, altura e bytes por pixels.
// ----------------------------

const (
	WIDTH  = 50
	HEIGHT = 50
	BYTES  = 4
)

// ------------------------------------------------------------
// Estruturas equivalentes às estruturas C da WinAPI.
//
// uintptr representa handles e ponteiros.
// int32 representa os inteiros de 32 bits usados pela API.
// ------------------------------------------------------------

type WNDCLASSEXW struct {
	CbSize     uint32
	Style      uint32
	WndProc    uintptr
	ClsExtra   int32
	WndExtra   int32
	Instance   uintptr
	Icon       uintptr
	Cursor     uintptr
	Background uintptr
	MenuName   *uint16
	ClassName  *uint16
	IconSm     uintptr
}

type POINT struct {
	X int32
	Y int32
}

type MSG struct {
	Hwnd    uintptr
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      POINT
}

type PAINTSTRUCT struct {
	Hdc         uintptr
	Erase       int32
	RcPaint     RECT
	Restore     int32
	IncUpdate   int32
	RgbReserved [32]byte
}

type RECT struct {
	Left   int32
	Top    int32
	Right  int32
	Bottom int32
}

// ------------------------------------------------------------
// WndProc: função chamada pelo Windows quando uma mensagem
// precisa ser processada pela nossa janela.
//
// Os parâmetros são uintptr porque representam os tipos
// HWND, WPARAM e LPARAM da API Win32.
// ------------------------------------------------------------

func wndProc(
	hwnd, message, wParam, lParam uintptr,
) uintptr {

	switch uint32(message) {

	case WM_PAINT:

		var ps PAINTSTRUCT

		hbrush, _, _ := procCreateSolidBrush.Call(0, 0, 0)

		hdc, _, _ := procBeginPaint.Call(
			hwnd,
			uintptr(unsafe.Pointer(&ps)),
		)

		procFillRect.Call(hdc, uintptr(unsafe.Pointer(&ps.RcPaint)), (hbrush))

		procDeleteObject.Call(hbrush)

		procEndPaint.Call(
			hwnd,
			uintptr(unsafe.Pointer(&ps)),
		)

	case WM_DESTROY:
		// O usuário fechou a janela.
		// Pedimos que o message loop termine.
		procPostQuitMessage.Call(0)
		return 0
	}

	// Delega mensagens não tratadas ao Windows.
	result, _, _ := procDefWindowProcW.Call(
		hwnd,
		message,
		wParam,
		lParam,
	)

	return result
}

// ------------------------------------------------------------
// main: registra a classe, cria a janela e processa mensagens.
// ------------------------------------------------------------

func main() {
	// Mantém a goroutine principal na mesma thread do SO.
	// Isso é importante para o gerenciamento de mensagens Win32.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	// Converte strings Go em strings UTF-16 terminadas em NUL.
	className, err := syscall.UTF16PtrFromString("MinhaJanelaGo")
	if err != nil {
		panic(err)
	}

	windowTitle, err := syscall.UTF16PtrFromString(
		"Nostos Engine",
	)
	if err != nil {
		panic(err)
	}

	// Obtém o HINSTANCE do executável atual.
	instance, _, callErr := procGetModuleHandleW.Call(0)
	if instance == 0 {
		panic(fmt.Sprintf("GetModuleHandleW: %v", callErr))
	}

	// Cria o ponteiro de callback para nossa WndProc.
	// A variável global da função continua acessível durante
	// toda a execução do programa.
	callback := syscall.NewCallback(wndProc)

	// Define como a janela será registrada no Windows.
	wc := WNDCLASSEXW{
		CbSize:    uint32(unsafe.Sizeof(WNDCLASSEXW{})),
		Style:     CS_HREDRAW | CS_VREDRAW,
		WndProc:   callback,
		Instance:  instance,
		ClassName: className,
	}

	// Registra a classe de janela.
	atom, _, callErr := procRegisterClassExW.Call(
		uintptr(unsafe.Pointer(&wc)),
	)
	if atom == 0 {
		panic(fmt.Sprintf("RegisterClassExW: %v", callErr))
	}

	// Cria uma janela de nível superior.
	// CW_USEDEFAULT deixa o Windows escolher posição e tamanho
	// inicial padrão, exceto pela largura e altura informadas.
	hwnd, _, callErr := procCreateWindowExW.Call(
		0, // dwExStyle
		uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(windowTitle)),
		WS_OVERLAPPEDWINDOW,
		uintptr(uint32(CW_USEDEFAULT)), // X
		uintptr(uint32(CW_USEDEFAULT)), // Y
		800,                            // largura
		600,                            // altura
		0,                              // janela pai
		0,                              // menu
		instance,                       // HINSTANCE
		0,                              // lpParam
	)

	if hwnd == 0 {
		panic(fmt.Sprintf("CreateWindowExW: %v", callErr))
	}

	// Exibe a janela e solicita uma atualização inicial.
	procShowWindow.Call(hwnd, SW_SHOW)
	procUpdateWindow.Call(hwnd)

	// Message loop:
	// GetMessageW retorna:
	//   > 0: mensagem recebida
	//   = 0: mensagem WM_QUIT
	//   = -1: erro
	var msg MSG
	framebuffer := make([]byte, WIDTH*HEIGHT*BYTES)

	for {

		result, _, callErr := procGetMessageW.Call(
			uintptr(unsafe.Pointer(&msg)),
			0, // qualquer janela da thread
			0, // sem filtro de mensagem mínima
			0, // sem filtro de mensagem máxima
		)

		if int32(result) == -1 {
			panic(fmt.Sprintf("GetMessageW: %v", callErr))
		}

		if result == 0 {
			// WM_QUIT: encerra o programa.
			break
		}

		// Traduz mensagens de teclado, quando aplicável.
		procTranslateMessage.Call(
			uintptr(unsafe.Pointer(&msg)),
		)

		// Encaminha a mensagem à WndProc da janela.
		procDispatchMessageW.Call(
			uintptr(unsafe.Pointer(&msg)),
		)
	}
}
