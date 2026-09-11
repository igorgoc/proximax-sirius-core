param(
    [string]$Title = "Select Sirius Blockchain Data Directory",
    [string]$Mode = "dir"
)

Add-Type -TypeDefinition @"
using System;
using System.Windows.Forms;
using System.Runtime.InteropServices;

public class Win32Picker {
    [DllImport("user32.dll")]
    public static extern IntPtr GetForegroundWindow();

    [DllImport("user32.dll")]
    public static extern bool SetForegroundWindow(IntPtr hWnd);

    [ComImport, Guid("DC1C5A9C-E88A-4dde-A5A1-60F82A20AEF7")]
    [ClassInterface(ClassInterfaceType.None)]
    private class FileOpenDialogRc {}

    [ComImport, Guid("d57c7288-d4ad-4768-be02-9d969532d960"), InterfaceType(ComInterfaceType.InterfaceIsIUnknown)]
    private interface IFileOpenDialog {
        [PreserveSig] int Show(IntPtr parent);
        void SetFileTypes();
        void SetFileTypeIndex();
        void GetFileTypeIndex();
        void Advise();
        void Unadvise();
        void SetOptions(uint fos);
        void GetOptions(out uint fos);
        void SetDefaultFolder();
        void SetFolder(IntPtr psi);
        void GetFolder();
        void GetCurrentSelection();
        void SetFileName([MarshalAs(UnmanagedType.LPWStr)] string pszName);
        void GetFileName();
        void SetTitle([MarshalAs(UnmanagedType.LPWStr)] string pszTitle);
        void SetOkButtonLabel([MarshalAs(UnmanagedType.LPWStr)] string pszText);
        void SetFileNameLabel([MarshalAs(UnmanagedType.LPWStr)] string pszLabel);
        void GetResult(out IntPtr ppsi);
    }

    [ComImport, Guid("43826d1e-e718-42ee-bc55-a1e261c37bfe"), InterfaceType(ComInterfaceType.InterfaceIsIUnknown)]
    private interface IShellItem {
        void BindToHandler();
        void GetParent();
        void GetDisplayName(uint sigdnName, [MarshalAs(UnmanagedType.LPWStr)] out string ppszName);
        void GetAttributes();
        void Compare();
    }

    public static string PickFolder(string title, IntPtr hwnd) {
        var dialog = (IFileOpenDialog)new FileOpenDialogRc();
        uint options;
        dialog.GetOptions(out options);
        dialog.SetOptions(options | 0x00000020 | 0x00000040); // FOS_PICKFOLDERS | FOS_FORCEFILESYSTEM
        if (!string.IsNullOrEmpty(title)) {
            dialog.SetTitle(title);
        }
        int hr = dialog.Show(hwnd);
        if (hr != 0 && hwnd != IntPtr.Zero) {
            hr = dialog.Show(IntPtr.Zero);
        }
        if (hr == 0) {
            IntPtr ppsi;
            dialog.GetResult(out ppsi);
            var item = (IShellItem)Marshal.GetObjectForIUnknown(ppsi);
            string path;
            item.GetDisplayName(0x80028000, out path); // SIGDN_FILESYSPATH
            Marshal.Release(ppsi);
            return path;
        }
        return null;
    }

    public static string PickFile(string title, IntPtr hwnd) {
        var dialog = (IFileOpenDialog)new FileOpenDialogRc();
        uint options;
        dialog.GetOptions(out options);
        dialog.SetOptions(options | 0x00000040 | 0x00001000); // FOS_FORCEFILESYSTEM | FOS_FILEMUSTEXIST
        if (!string.IsNullOrEmpty(title)) {
            dialog.SetTitle(title);
        }
        int hr = dialog.Show(hwnd);
        if (hr != 0 && hwnd != IntPtr.Zero) {
            hr = dialog.Show(IntPtr.Zero);
        }
        if (hr == 0) {
            IntPtr ppsi;
            dialog.GetResult(out ppsi);
            var item = (IShellItem)Marshal.GetObjectForIUnknown(ppsi);
            string path;
            item.GetDisplayName(0x80028000, out path);
            Marshal.Release(ppsi);
            return path;
        }
        return null;
    }
}
"@ -ReferencedAssemblies "System.Windows.Forms" -ErrorAction SilentlyContinue

Add-Type -AssemblyName System.Windows.Forms
$owner = New-Object System.Windows.Forms.Form
$owner.TopMost = $true
$owner.StartPosition = [System.Windows.Forms.FormStartPosition]::CenterScreen
$owner.ShowInTaskbar = $false
$owner.Opacity = 0
$owner.Show()

try {
    if ($Mode -eq "file") {
        $result = [Win32Picker]::PickFile($Title, $owner.Handle)
    } else {
        $result = [Win32Picker]::PickFolder($Title, $owner.Handle)
    }
    if ($result) {
        [Console]::Out.Write($result)
        exit 0
    }
} catch {
    # Fallback to standard WinForms with top-most owner
    if ($Mode -eq "file") {
        $f = New-Object System.Windows.Forms.OpenFileDialog
        $f.Title = $Title
        if ($f.ShowDialog($owner) -eq [System.Windows.Forms.DialogResult]::OK) {
            [Console]::Out.Write($f.FileName)
        }
    } else {
        $f = New-Object System.Windows.Forms.FolderBrowserDialog
        $f.Description = $Title
        $f.RootFolder = [System.Environment+SpecialFolder]::MyComputer
        if ($f.ShowDialog($owner) -eq [System.Windows.Forms.DialogResult]::OK) {
            [Console]::Out.Write($f.SelectedPath)
        }
    }
} finally {
    $owner.Dispose()
}
