Set WshShell = CreateObject("WScript.Shell")
Set fso = CreateObject("Scripting.FileSystemObject")

' Dapatkan direktori root proyek (satu tingkat di atas folder scripts)
scriptDir = fso.GetParentFolderName(WScript.ScriptFullName)
rootDir = fso.GetParentFolderName(scriptDir)

' Set working directory ke root proyek
WshShell.CurrentDirectory = rootDir

' Jalankan refresh_token.py secara 100% invisible (window style 0, no wait)
cmd = "python refresh_token.py"
WshShell.Run cmd, 0, False
