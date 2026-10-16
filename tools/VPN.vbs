' Запускает клиент без окна консоли (значок появится в трее, откроется браузер).
Set fso = CreateObject("Scripting.FileSystemObject")
Set sh  = CreateObject("WScript.Shell")
dir = fso.GetParentFolderName(WScript.ScriptFullName)
sh.Run """" & dir & "\vpn-client.exe""", 0, False
