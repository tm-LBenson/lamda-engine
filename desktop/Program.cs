using System;
using System.Drawing;
using System.IO;
using System.Reflection;
using System.Runtime.InteropServices;
using System.Threading;
using System.Windows.Forms;

[assembly: AssemblyTitle("LamdaUI")]
[assembly: AssemblyDescription("LamdaUI companion control panel")]
[assembly: AssemblyCompany("Lewis Benson")]
[assembly: AssemblyProduct("LamdaUI")]
[assembly: AssemblyVersion("0.5.1.0")]
[assembly: AssemblyFileVersion("0.5.1.0")]
[assembly: System.Runtime.Versioning.TargetFramework(".NETFramework,Version=v4.8")]

namespace LamdaUI {
    internal static class Program {
        [DllImport("shell32.dll", CharSet = CharSet.Unicode)] private static extern int SetCurrentProcessExplicitAppUserModelID(string id);
        [STAThread]
        public static int Main(string[] args) {
            Application.EnableVisualStyles(); Application.SetCompatibleTextRenderingDefault(false);
            SetCurrentProcessExplicitAppUserModelID("LamdaUI.ControlPanel");
            try {
                if (args.Length == 2 && args[0] == "--self-test") return DesktopTests.Run(args[1]);
                if (args.Length == 2 && args[0] == "--render-test") return DesktopTests.Render(args[1]);
                if (args.Length == 1 && args[0] == "--gate-probe") return Gate.Busy() ? 3 : 0;
                bool hidden = false, quit = false; string start = "";
                for (int i = 0; i < args.Length; i++) {
                    if (args[i] == "--hidden") hidden = true;
                    else if (args[i] == "--quit") quit = true;
                    else if (args[i] == "--start" && i + 1 < args.Length && (args[i + 1] == "retail" || args[i + 1] == "forever")) start = args[++i];
                    else throw new ArgumentException("Unknown LamdaUI command.");
                }
                var signals = new EventWaitHandle[4];
                string[] names = { "Show", "StartRetail", "StartForever", "Quit" };
                try {
                    for (int i = 0; i < signals.Length; i++) signals[i] = new EventWaitHandle(false, EventResetMode.AutoReset, @"Local\LamdaUI.Panel." + names[i]);
                    bool created;
                    using (var singleton = new Mutex(true, @"Local\LamdaUI.ControlPanel", out created)) {
                        if (!created) {
                            if (quit) signals[3].Set();
                            else { if (!hidden) signals[0].Set(); if (start == "retail") signals[1].Set(); if (start == "forever") signals[2].Set(); }
                            return 0;
                        }
                        try {
                            if (quit) return 0;
                            string root = Directory.GetParent(AppDomain.CurrentDomain.BaseDirectory.TrimEnd('\\')).FullName;
                            using (var app = new TrayApp(root, signals, hidden, start)) Application.Run(app);
                        } finally { singleton.ReleaseMutex(); }
                    }
                } finally { foreach (var signal in signals) if (signal != null) signal.Dispose(); }
                return 0;
            } catch (Exception error) {
                try { File.AppendAllText(Path.Combine(AppDomain.CurrentDomain.BaseDirectory, "startup-error.log"), DateTime.Now.ToString("s") + " " + error + Environment.NewLine); } catch { }
                if (args.Length > 0 && (args[0] == "--self-test" || args[0] == "--render-test" || args[0] == "--gate-probe" || args[0] == "--quit")) return 1;
                MessageBox.Show("LamdaUI could not open.\n\n" + error.Message, "LamdaUI", MessageBoxButtons.OK, MessageBoxIcon.Error);
                return 1;
            }
        }
    }
}
