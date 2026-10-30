import AppKit
import UserNotifications

// A synthetic notification helper, not a monitor or an installed service.
let usage = "Usage: bambu-notify --test [--sound] | --status | --deliver | --help"
let arguments = Array(CommandLine.arguments.dropFirst())
if arguments.isEmpty || arguments == ["--help"] {
    print(usage)
    exit(0)
}
let statusOnly = arguments == ["--status"]
let delivery = arguments == ["--deliver"]
struct Delivery: Decodable { let title: String?; let body: String; let sound: Bool }
var title = "Bambu Stand"
var body = "Branded helper test. No printer event occurred."
var sound = arguments.contains("--sound")
if delivery {
    let data = FileHandle.standardInput.readData(ofLength: 4097)
    guard data.count <= 4096, let message = try? JSONDecoder().decode(Delivery.self, from: data), !message.body.isEmpty, message.body.count <= 256, message.title == nil || (!(message.title?.isEmpty ?? true) && (message.title?.count ?? 0) <= 128) else {
        fputs("Invalid notification input.\n", stderr)
        exit(2)
    }
    title = message.title ?? "Bambu Stand"
    body = message.body
    sound = message.sound
}
guard delivery || statusOnly || arguments == ["--test"] || arguments == ["--test", "--sound"] else {
    fputs(usage + "\n", stderr)
    exit(2)
}
guard Bundle.main.bundleIdentifier == "com.github.cliffom.bambu-tui.notify-poc" else {
    fputs("Run the executable inside the built Bambu Stand Notify.app bundle.\n", stderr)
    exit(2)
}
if ["SSH_CONNECTION", "SSH_CLIENT", "SSH_TTY"].contains(where: {
    !(ProcessInfo.processInfo.environment[$0] ?? "").isEmpty
}) {
    fputs("Run this helper from a local macOS desktop session; SSH is unsupported.\n", stderr)
    exit(2)
}

func finish(_ code: Int32, _ message: String) -> Never {
    if code == 0 {
        print(message)
        fflush(stdout)
    } else {
        fputs(message + "\n", stderr)
    }
    exit(code)
}

func authorizationName(_ value: UNAuthorizationStatus) -> String {
    switch value {
    case .notDetermined: return "not determined"
    case .denied: return "denied"
    case .authorized: return "authorized"
    case .provisional: return "provisional"
    case .ephemeral: return "ephemeral"
    @unknown default: return "unknown"
    }
}

func previewName(_ value: UNShowPreviewsSetting) -> String {
    switch value {
    case .always: return "always"
    case .whenAuthenticated: return "when unlocked"
    case .never: return "never"
    @unknown default: return "unknown"
    }
}

func notificationFailure(_ error: Error, operation: String) -> Never {
    let native = error as NSError
    if native.domain == UNErrorDomain && native.code == UNError.Code.notificationsNotAllowed.rawValue {
        finish(3, "Notifications are not allowed. Enable Bambu Stand in System Settings → Notifications, then retry --test.")
    }
    finish(4, "\(operation) failed: \(error.localizedDescription)")
}

final class NotificationDelegate: NSObject, UNUserNotificationCenterDelegate {
    func userNotificationCenter(
        _ center: UNUserNotificationCenter,
        willPresent notification: UNNotification,
        withCompletionHandler completionHandler: @escaping (UNNotificationPresentationOptions) -> Void
    ) {
        var options: UNNotificationPresentationOptions = [.banner, .list]
        if notification.request.content.sound != nil {
            options.insert(.sound)
        }
        completionHandler(options)
    }
}

let app = NSApplication.shared
app.setActivationPolicy(.accessory)
let center = UNUserNotificationCenter.current()
let delegate = NotificationDelegate()
center.delegate = delegate

// Includes time for the user to answer the first-run permission prompt.
DispatchQueue.main.asyncAfter(deadline: .now() + (delivery ? 8 : 60)) {
    finish(5, "Timed out waiting for macOS notification permission/service. No monitoring remains running.")
}

if statusOnly {
    center.getNotificationSettings { settings in
        finish(0, "Authorization: \(authorizationName(settings.authorizationStatus)); alerts: \(settings.alertSetting.rawValue); sound: \(settings.soundSetting.rawValue) (0=unsupported, 1=disabled, 2=enabled); previews: \(previewName(settings.showPreviewsSetting)).")
    }
} else {
    let submit: () -> Void = {
        let content = UNMutableNotificationContent()
        content.title = title
        content.body = body
        if sound {
            content.sound = .default
        }
        // Immediate delivery only: nothing is scheduled to fire after this probe exits.
        let request = UNNotificationRequest(identifier: UUID().uuidString, content: content, trigger: nil)
        center.add(request) { error in
            if let error = error {
                notificationFailure(error, operation: "Notification submission")
            }
            DispatchQueue.main.asyncAfter(deadline: .now() + (delivery ? 0 : 1)) {
                finish(0, "macOS accepted the branded notification request. Confirm the logo/banner and sound yourself. The helper is exiting; no monitor or service was started.")
            }
        }
    }
    if delivery {
        center.getNotificationSettings { settings in
            guard settings.authorizationStatus == .authorized else {
                finish(3, "Notifications are not authorized. Run bambustand doctor --notify first.")
            }
            submit()
        }
    } else {
        print("Bambu Stand notification test: allow notifications if macOS prompts. Sound requested: \(sound).")
        fflush(stdout)
        center.requestAuthorization(options: [.alert, .sound]) { granted, error in
            if let error = error { notificationFailure(error, operation: "Notification authorization") }
            guard granted else { finish(3, "Notifications denied. Enable Bambu Stand in System Settings → Notifications, then retry.") }
            submit()
        }
    }
}
app.run()
