using System;
using System.Collections.Generic;
using System.Diagnostics;
using System.Drawing;
using System.IO;
using System.Linq;
using System.Runtime.InteropServices;
using System.Threading;
using System.Threading.Tasks;
using System.Windows.Forms;

namespace LamdaUI {
    internal sealed class ThemeButton : Button {
        private bool hover, pressed;
        public ThemeButton() { SetStyle(ControlStyles.UserPaint | ControlStyles.AllPaintingInWmPaint | ControlStyles.OptimizedDoubleBuffer, true); }
        protected override void OnMouseEnter(EventArgs e) { hover = true; Invalidate(); base.OnMouseEnter(e); }
        protected override void OnMouseLeave(EventArgs e) { hover = pressed = false; Invalidate(); base.OnMouseLeave(e); }
        protected override void OnMouseDown(MouseEventArgs e) { pressed = true; Invalidate(); base.OnMouseDown(e); }
        protected override void OnMouseUp(MouseEventArgs e) { pressed = false; Invalidate(); base.OnMouseUp(e); }
        protected override void OnPaint(PaintEventArgs e) {
            Color background = Enabled && pressed ? FlatAppearance.MouseDownBackColor : Enabled && hover ? FlatAppearance.MouseOverBackColor : BackColor;
            e.Graphics.Clear(background);
            using (var border = new Pen(FlatAppearance.BorderColor)) e.Graphics.DrawRectangle(border, 0, 0, Width - 1, Height - 1);
            TextRenderer.DrawText(e.Graphics, Text, Font, ClientRectangle, Enabled ? ForeColor : Color.FromArgb(113, 137, 156),
                                 TextFormatFlags.HorizontalCenter | TextFormatFlags.VerticalCenter | TextFormatFlags.EndEllipsis | TextFormatFlags.NoPrefix);
            if (Focused && ShowFocusCues) ControlPaint.DrawFocusRectangle(e.Graphics, Rectangle.Inflate(ClientRectangle, -4, -4), Palette.Accent, background);
        }
    }
    internal static class Palette {
        public static readonly Color Background = Color.FromArgb(11, 18, 27), Surface = Color.FromArgb(18, 29, 41),
            Border = Color.FromArgb(43, 61, 77), Text = Color.FromArgb(231, 240, 248), Muted = Color.FromArgb(151, 171, 190),
            Accent = Color.FromArgb(84, 212, 215), Amber = Color.FromArgb(240, 190, 112);
        public static Label Label(Control parent, string text, int x, int y, int width, int height, float size, bool bold) {
            var label = new Label { Text = text, Location = new Point(x, y), Size = new Size(width, height),
                Font = new Font("Segoe UI", size, bold ? FontStyle.Bold : FontStyle.Regular), ForeColor = Text,
                BackColor = Color.Transparent, AutoEllipsis = true, UseMnemonic = false };
            parent.Controls.Add(label); return label;
        }
        public static Button Button(Control parent, string text, int x, int y, int width, Action action) {
            var button = new ThemeButton { Text = text, Location = new Point(x, y), Size = new Size(width, 32),
                FlatStyle = FlatStyle.Flat, BackColor = Color.FromArgb(27, 46, 62), ForeColor = Text,
                Font = new Font("Segoe UI", 9.5f, FontStyle.Bold), Cursor = Cursors.Hand, UseVisualStyleBackColor = false };
            button.FlatAppearance.BorderColor = Border;
            button.FlatAppearance.MouseOverBackColor = Color.FromArgb(35, 65, 79);
            button.FlatAppearance.MouseDownBackColor = Color.FromArgb(40, 84, 93);
            button.Click += delegate { action(); };
            parent.Controls.Add(button); return button;
        }
    }
    internal sealed class Card : System.Windows.Forms.Panel {
        public Card(int y, int height) {
            SetStyle(ControlStyles.OptimizedDoubleBuffer | ControlStyles.ResizeRedraw, true);
            Location = new Point(24, y); Size = new Size(592, height); BackColor = Palette.Surface;
        }
        protected override void OnPaint(PaintEventArgs e) {
            base.OnPaint(e);
            using (var pen = new Pen(Palette.Border)) e.Graphics.DrawRectangle(pen, 0, 0, Width - 1, Height - 1);
        }
    }
    internal sealed class ClientCard {
        public readonly Card View;
        public readonly Label State, Versions, Detail;
        public readonly Button Start, Restart, Stop;
        public ClientCard(Client client, int y, Action<string> command, Action browse, Action logs) {
            View = new Card(y, 154);
            Palette.Label(View, client.Name, 20, 14, 160, 28, 15, true);
            State = Palette.Label(View, "Checking...", 180, 19, 390, 24, 10, true); State.TextAlign = ContentAlignment.TopRight;
            Versions = Palette.Label(View, "", 20, 49, 550, 20, 9, false); Versions.ForeColor = Palette.Muted;
            Detail = Palette.Label(View, "", 20, 75, 550, 24, 10, false);
            Start = Palette.Button(View, "Start", 20, 107, 94, () => command("start"));
            Restart = Palette.Button(View, "Restart", 124, 107, 94, () => command("restart"));
            Stop = Palette.Button(View, "Stop", 228, 107, 94, () => command("stop"));
            Palette.Button(View, "Addons", 332, 107, 102, browse);
            Palette.Button(View, "Open logs", 444, 107, 126, logs);
        }
        public void Bind(EngineState state, bool busy) {
            State.Text = "● " + state.Headline; State.ForeColor = state.Color;
            Versions.Text = "Engine " + (state.Version == "" ? "—" : state.Version) + "    ·    Addon " + (state.AddonVersion == "" ? "—" : state.AddonVersion);
            Detail.Text = state.Detail;
            Start.Enabled = !busy && state.Installed && !state.Unknown && !state.Running;
            Restart.Enabled = Stop.Enabled = !busy && state.Running && !state.Unknown;
        }
    }
    internal sealed class Dashboard : Form {
        [DllImport("dwmapi.dll")] private static extern int DwmSetWindowAttribute(IntPtr window, int attribute, ref int value, int size);
        public readonly Dictionary<string, ClientCard> Cards = new Dictionary<string, ClientCard>();
        public readonly Label UpdateStatus, Footer;
        public readonly Button UpdateButton;
        public bool Quitting;
        public bool RenderOnly;
        protected override bool ShowWithoutActivation { get { return RenderOnly; } }
        public Dashboard(Client[] clients, Icon icon, Action<Client, string> action, Action<Client> browse, Action<Client> logs, Action update, Action updateLogs) {
            Text = "LamdaUI"; Icon = icon; BackColor = Palette.Background; ForeColor = Palette.Text;
            Font = new Font("Segoe UI", 10); AutoScaleDimensions = new SizeF(96, 96); AutoScaleMode = AutoScaleMode.Dpi;
            ClientSize = new Size(640, 680); FormBorderStyle = FormBorderStyle.FixedSingle; MaximizeBox = false;
            StartPosition = FormStartPosition.CenterScreen;
            var logo = new PictureBox { Location = new Point(24, 24), Size = new Size(46, 46), SizeMode = PictureBoxSizeMode.Zoom, Image = icon.ToBitmap() };
            Controls.Add(logo);
            Palette.Label(this, "LamdaUI", 86, 16, 265, 43, 25, true);
            var subtitle = Palette.Label(this, "Retail + Forever companion", 89, 61, 330, 22, 10, false); subtitle.ForeColor = Palette.Muted;
            var tag = Palette.Label(this, "COMPANION CONTROL", 410, 37, 204, 20, 8.5f, true); tag.TextAlign = ContentAlignment.TopRight; tag.ForeColor = Palette.Accent;
            int y = 100;
            foreach (Client client in clients) {
                Client selected = client;
                var card = new ClientCard(selected, y, cmd => action(selected, cmd), () => browse(selected), () => logs(selected));
                Cards.Add(selected.Key, card); Controls.Add(card.View); y += 168;
            }
            var updates = new Card(436, 124); Controls.Add(updates);
            Palette.Label(updates, "Updates", 20, 14, 300, 28, 14, true);
            var local = Palette.Label(updates, "RELEASE UPDATES", 382, 21, 188, 20, 8, true); local.TextAlign = ContentAlignment.TopRight; local.ForeColor = Palette.Muted;
            UpdateStatus = Palette.Label(updates, "Checking update source...", 20, 46, 550, 24, 9.5f, false);
            UpdateButton = Palette.Button(updates, "Check for updates", 20, 78, 166, update);
            Palette.Button(updates, "Update logs", 198, 78, 128, updateLogs);
            var wait = Palette.Label(updates, "Keep playing; /reload when ready.", 333, 86, 250, 22, 9, false); wait.ForeColor = Palette.Muted;
            Footer = Palette.Label(this, "Frames and heal binds work without the companion.", 24, 579, 592, 23, 9.5f, false); Footer.ForeColor = Palette.Muted;
            var hint = Palette.Label(this, "Closing this window keeps LamdaUI in the tray.", 24, 604, 592, 22, 9, false); hint.ForeColor = Palette.Muted;
            Palette.Button(this, "Copy /lamda", 24, 638, 138, delegate { Clipboard.SetText("/lamda"); Footer.Text = "Copied /lamda. Paste it into WoW chat to open your settings."; Footer.ForeColor = Palette.Accent; });
            Palette.Button(this, "Hide to tray", 478, 638, 138, Hide);
            FormClosing += delegate(object sender, FormClosingEventArgs e) {
                if (!Quitting && e.CloseReason == CloseReason.UserClosing) { e.Cancel = true; Hide(); }
            };
            Resize += delegate { if (WindowState == FormWindowState.Minimized) Hide(); };
            HandleCreated += delegate {
                try { int value = 1; DwmSetWindowAttribute(Handle, 20, ref value, sizeof(int)); } catch { }
            };
            Disposed += delegate { logo.Image.Dispose(); };
        }
        public void Open() { Show(); WindowState = FormWindowState.Normal; Activate(); BringToFront(); }
    }

    internal sealed class TrayApp : ApplicationContext {
        private readonly Controller controller;
        private readonly Dashboard panel;
        private readonly NotifyIcon tray;
        private readonly Icon icon;
        private readonly System.Windows.Forms.Timer timer;
        private readonly EventWaitHandle[] requests;
        private readonly List<ToolStripMenuItem> engineItems = new List<ToolStripMenuItem>();
        private readonly ToolStripMenuItem updateItem, quitItem;
        private readonly Dictionary<string, EngineState> states = new Dictionary<string, EngineState>();
        private readonly SimNotifications simNotifications = new SimNotifications();
        private bool actionBusy, quitting, exitRequested;
        private DateTime nextRefresh = DateTime.MinValue;
        private Process updateProcess;
        public TrayApp(string root, EventWaitHandle[] requests, bool hidden, string start) {
            this.requests = requests; controller = new Controller(root);
            icon = Icon.ExtractAssociatedIcon(Application.ExecutablePath);
            panel = new Dashboard(controller.Clients, icon, BeginAction, c => OpenWorkspace(c, "/addons", "Browse Addons"), c => OpenFolder(Path.Combine(c.Home, "data")), BeginUpdate,
                                  () => OpenFolder(Path.Combine(controller.Updates, "desktop-logs")));
            // Create a handle for UI-thread continuations while the window is hidden.
            IntPtr handle = panel.Handle;
            tray = new NotifyIcon { Icon = icon, Text = "LamdaUI", Visible = true };
            var menu = new ContextMenuStrip { BackColor = Palette.Surface, ForeColor = Palette.Text, ShowImageMargin = false };
            var open = new ToolStripMenuItem("Open LamdaUI") { Font = new Font("Segoe UI", 9, FontStyle.Bold) };
            open.Click += delegate { panel.Open(); }; menu.Items.Add(open); menu.Items.Add(new ToolStripSeparator());
            foreach (var client in controller.Clients) {
                var selected = client;
                var parent = new ToolStripMenuItem(client.Name);
                foreach (string operation in new [] { "start", "restart", "stop" }) {
                    string cmd = operation;
                    var item = new ToolStripMenuItem(Char.ToUpper(cmd[0]) + cmd.Substring(1));
                    item.Tag = new string[] { selected.Key, cmd };
                    item.Click += delegate { BeginAction(selected, cmd); };
                    parent.DropDownItems.Add(item); engineItems.Add(item);
                }
                parent.DropDownItems.Add(new ToolStripSeparator());
                parent.DropDownItems.Add("Browse Addons", null, delegate { OpenWorkspace(selected, "/addons", "Browse Addons"); });
                parent.DropDownItems.Add("Overlay windows...", null, delegate {
                    try { controller.OpenOverlayWindows(selected); }
                    catch (Exception error) { controller.Log(error.ToString()); MessageBox.Show(panel, error.Message, "Overlay windows", MessageBoxButtons.OK, MessageBoxIcon.Information); }
                });
                parent.DropDownItems.Add("Open sim workspace", null, delegate { OpenWorkspace(selected, "/", "workspace"); });
                parent.DropDownItems.Add("Open logs", null, delegate { OpenFolder(Path.Combine(selected.Home, "data")); });
                menu.Items.Add(parent);
            }
            menu.Items.Add(new ToolStripSeparator());
            updateItem = new ToolStripMenuItem("Check for updates"); updateItem.Click += delegate { BeginUpdate(); }; menu.Items.Add(updateItem);
            menu.Items.Add("Open control-panel logs", null, delegate { OpenFolder(Path.Combine(controller.Root, "Desktop", "data")); });
            quitItem = new ToolStripMenuItem("Exit tray app (engines keep running)"); quitItem.Click += delegate { exitRequested = true; }; menu.Items.Add(quitItem);
            menu.Opening += delegate { RefreshState(); };
            tray.ContextMenuStrip = menu;
            tray.MouseClick += delegate(object sender, MouseEventArgs e) { if (e.Button == MouseButtons.Left) panel.Open(); };
            timer = new System.Windows.Forms.Timer { Interval = 350 };
            timer.Tick += delegate {
                if (requests[0].WaitOne(0)) panel.Open();
                if (requests[1].WaitOne(0)) BeginAction(controller.Clients[0], "start");
                if (requests[2].WaitOne(0)) BeginAction(controller.Clients[1], "start");
                if (requests[3].WaitOne(0)) exitRequested = true;
                if (exitRequested && !actionBusy) { ExitThread(); return; }
                if (DateTime.UtcNow >= nextRefresh) RefreshState();
            };
            timer.Start(); RefreshState();
            if (!hidden) panel.Open();
            if (start == "retail") requests[1].Set();
            if (start == "forever") requests[2].Set();
        }
        private bool Updating() {
            if (updateProcess == null) return false;
            if (!updateProcess.HasExited) return true;
            int code = updateProcess.ExitCode; updateProcess.Dispose(); updateProcess = null;
            panel.Footer.Text = code == 0 ? "Update check finished. Use /reload when ready if your addon was updated." :
                code == 2 ? "This update needs WoW closed once. Click Update after you finish playing." :
                "The update stopped. Open update logs for the result.";
            panel.Footer.ForeColor = code == 0 ? Palette.Accent : Palette.Amber;
            return false;
        }
        private void RefreshState() {
            try {
                bool updating = Updating(), gateBusy = Gate.Busy(), busy = actionBusy || updating || gateBusy;
                foreach (var client in controller.Clients) {
                    states[client.Key] = controller.Read(client);
                    panel.Cards[client.Key].Bind(states[client.Key], busy);
                    if (states[client.Key].Healthy && !busy) {
                        try {
                            var notice = simNotifications.Observe(Files.Json(Path.Combine(client.Home, "data", "sim", "game-status.json")), client.Key, DateTime.UtcNow);
                            if (notice != null) {
                                panel.Footer.Text = client.Name + ": " + notice;
                                panel.Footer.ForeColor = Palette.Accent;
                                tray.ShowBalloonTip(10000, "LamdaUI · " + client.Name, notice, ToolTipIcon.Info);
                            }
                        } catch (IOException) { } catch (UnauthorizedAccessException) { } catch (ArgumentException) { }
                    }
                }
                foreach (var item in engineItems) {
                    var data = (string[])item.Tag; var state = states[data[0]];
                    item.Enabled = !busy && state.Installed && !state.Unknown && (data[1] == "start" ? !state.Running : state.Running);
                }
                panel.UpdateButton.Enabled = updateItem.Enabled = !busy;
                quitItem.Enabled = !actionBusy;
                panel.UpdateStatus.Text = actionBusy ? "Applying your engine control..." : controller.UpdateMessage(gateBusy || updating);
                panel.UpdateButton.Text = (gateBusy || updating) && !actionBusy ? "Updater running" : "Check for updates";
                string tip = "LamdaUI\nRetail: " + Short(states["retail"]) + "\nForever: " + Short(states["forever"]);
                tray.Text = tip.Length > 63 ? tip.Substring(0, 63) : tip;
                controller.PanelStatus(tray.Visible, panel.Visible, states, busy);
            } catch (Exception error) {
                controller.Log(error.ToString()); panel.Footer.Text = "Could not refresh status. Open control-panel logs for details."; panel.Footer.ForeColor = Palette.Amber;
            }
            nextRefresh = DateTime.UtcNow.AddSeconds(3);
        }
        private static string Short(EngineState state) { return state.Unknown ? "Unknown" : state.Running ? (state.Healthy ? "Running" : "Waiting") : "Stopped"; }
        private async void BeginAction(Client client, string action) {
            if (actionBusy || Updating() || Gate.Busy()) { ShowError("The updater or another engine control is busy."); return; }
            if (action != "stop") {
                try { if (!controller.ChooseAccount(client, panel)) return; }
                catch (Exception error) { controller.Log(error.ToString()); ShowError("Could not read your WoW accounts. Check that the game folder is available."); return; }
            }
            actionBusy = true; RefreshState();
            try {
                await Task.Run(() => controller.Execute(client, action));
                panel.Footer.Text = client.Name + ": " + (action == "stop" ? "companion stopped. Frames and heal binds remain available." : "companion is running.");
                panel.Footer.ForeColor = Palette.Accent;
            } catch (Exception error) { controller.Log(client.Name + " " + action + ": " + error); ShowError(client.Name + " could not " + action + ". Open control-panel logs for details."); }
            finally { actionBusy = false; RefreshState(); }
        }
        private void BeginUpdate() {
            if (actionBusy || Updating() || Gate.Busy()) return;
            try { updateProcess = controller.Update(); panel.Footer.Text = "The updater is open. You can keep playing during compatible updates."; panel.Footer.ForeColor = Palette.Muted; RefreshState(); }
            catch (Exception error) { controller.Log(error.ToString()); ShowError("Could not open the updater. Open control-panel logs for details."); }
        }
        private void ShowError(string message) {
            panel.Footer.Text = message; panel.Footer.ForeColor = Palette.Amber;
            if (!panel.Visible) tray.ShowBalloonTip(6000, "LamdaUI", message, ToolTipIcon.Warning);
        }
        private void OpenFolder(string folder) {
            try { Directory.CreateDirectory(folder); Process.Start("explorer.exe", Files.Quote(folder)); }
            catch (Exception error) { controller.Log(error.ToString()); ShowError("Could not open the logs folder."); }
        }
        private void OpenWorkspace(Client client, string path, string name) {
            try {
                EngineState state;
                if (!states.TryGetValue(client.Key, out state) || !state.Running) {
                    ShowError("Start the " + client.Name + " engine before opening " + name + "."); return;
                }
                string url = (client.Key == "forever" ? "http://127.0.0.1:47642" : "http://127.0.0.1:47641") + path;
                Process.Start(new ProcessStartInfo(url) { UseShellExecute = true });
            } catch (Exception error) { controller.Log(error.ToString()); ShowError("Could not open the local " + name + "."); }
        }
        protected override void ExitThreadCore() {
            if (quitting) return;
            quitting = true; timer.Stop(); timer.Dispose();
            tray.Visible = false; tray.ContextMenuStrip.Dispose(); tray.Dispose();
            panel.Quitting = true; panel.Close(); panel.Dispose(); icon.Dispose();
            if (updateProcess != null) updateProcess.Dispose(); // The updater continues independently.
            base.ExitThreadCore();
        }
    }
}
