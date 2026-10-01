import SwiftUI

// MARK: - Screen chrome

struct ThemedScreenModifier: ViewModifier {
    func body(content: Content) -> some View {
        content
            .background(Theme.background)
            .toolbarBackground(Theme.brandBg.opacity(0.94), for: .navigationBar)
            .toolbarBackground(.visible, for: .navigationBar)
            .toolbarColorScheme(.dark, for: .navigationBar)
    }
}

extension View {
    func themedScreen() -> some View {
        modifier(ThemedScreenModifier())
    }

    func themedList() -> some View {
        listStyle(.insetGrouped)
            .scrollContentBackground(.hidden)
            .background(Theme.background)
    }

    func themedForm() -> some View {
        scrollContentBackground(.hidden)
            .background(Theme.background)
    }
}

// MARK: - Interaction

/// Press feedback for custom (non-system) controls: a small scale plus a selection
/// haptic. Scale only — never layout — so held rows cannot nudge their neighbours.
/// Honours Reduce Motion by dropping the scale and keeping the haptic.
struct PressableStyle: ButtonStyle {
    @Environment(\.accessibilityReduceMotion) private var reduceMotion
    var scale: CGFloat = Theme.Motion.pressedScale

    func makeBody(configuration: Configuration) -> some View {
        configuration.label
            .scaleEffect(reduceMotion ? 1 : (configuration.isPressed ? scale : 1))
            .animation(Theme.Motion.quick, value: configuration.isPressed)
            .sensoryFeedback(.selection, trigger: configuration.isPressed)
    }
}

extension ButtonStyle where Self == PressableStyle {
    static var pressable: PressableStyle { PressableStyle() }
}

// MARK: - Cards & sections

struct SurfaceCard<Content: View>: View {
    var cornerRadius: CGFloat = Theme.cornerRadius
    @ViewBuilder let content: () -> Content

    var body: some View {
        content()
            .padding(Theme.spacingMD)
            .frame(maxWidth: .infinity, alignment: .leading)
            .background(Theme.surface, in: RoundedRectangle(cornerRadius: cornerRadius, style: .continuous))
            .overlay(
                RoundedRectangle(cornerRadius: cornerRadius, style: .continuous)
                    .strokeBorder(Theme.brandBorder, lineWidth: 1)
            )
            .elevation(Theme.Elevation.card)
    }
}

struct SectionHeader: View {
    let title: String
    var actionTitle: String?
    var action: (() -> Void)?

    var body: some View {
        HStack(alignment: .firstTextBaseline) {
            Text(title)
                .font(Theme.Typography.sectionTitle)
                .foregroundStyle(Theme.textPrimary)
            Spacer(minLength: Theme.spacingSM)
            if let actionTitle, let action {
                Button(actionTitle, action: action)
                    .font(.footnote.weight(.semibold))
                    .foregroundStyle(Theme.accentLight)
                    // Keeps a short word like "All" tappable at 44pt.
                    .frame(minHeight: Theme.minTouchTarget)
            }
        }
    }
}

/// All-caps label for `Form`/`List` section headers. SwiftUI styles the default
/// header with its own washed-out secondary colour, which is the main reason
/// headers read as unreadable on a dark canvas — this replaces it.
struct FormSectionHeader: View {
    let title: String

    var body: some View {
        Text(title.uppercased())
            .font(Theme.Typography.cardLabel)
            .tracking(0.8)
            .foregroundStyle(Theme.muted)
    }
}

struct ClusterContextBanner: View {
    @Environment(AppState.self) private var appState

    var body: some View {
        HStack(spacing: Theme.spacingSM) {
            Image(systemName: "server.rack")
                .font(.title3)
                .foregroundStyle(Theme.accentLight)
                .symbolRenderingMode(.hierarchical)
                .frame(width: 28)

            VStack(alignment: .leading, spacing: 2) {
                Text(appState.authManager.activeAccount?.displayName ?? "Cluster")
                    .font(.subheadline.weight(.semibold))
                    .foregroundStyle(Theme.textPrimary)
                Text(appState.clusterManager.activeContextName)
                    .font(Theme.Typography.identifierSmall)
                    .foregroundStyle(Theme.muted)
                    .lineLimit(1)
                    .truncationMode(.middle)
            }

            Spacer(minLength: Theme.spacingSM)

            if !appState.clusterManager.selectedNamespace.isEmpty {
                Text(appState.clusterManager.selectedNamespace)
                    .font(Theme.Typography.identifierSmall.weight(.semibold))
                    .lineLimit(1)
                    .padding(.horizontal, 10)
                    .padding(.vertical, 5)
                    .background(Theme.accent.opacity(0.18), in: Capsule())
                    .overlay(Capsule().strokeBorder(Theme.accent.opacity(0.45), lineWidth: 1))
                    .foregroundStyle(Theme.accentLight)
                    .accessibilityLabel("Namespace \(appState.clusterManager.selectedNamespace)")
            }
        }
        .padding(Theme.spacingMD)
        .background(Theme.surface.opacity(0.75), in: RoundedRectangle(cornerRadius: Theme.cornerRadiusSmall, style: .continuous))
        .background(.ultraThinMaterial, in: RoundedRectangle(cornerRadius: Theme.cornerRadiusSmall, style: .continuous))
        .overlay(
            RoundedRectangle(cornerRadius: Theme.cornerRadiusSmall, style: .continuous)
                .strokeBorder(Theme.brandBorder, lineWidth: 1)
        )
    }
}

// MARK: - Metrics

struct MetricCard: View {
    let title: String
    let value: String
    let subtitle: String?
    let tint: Color
    let action: (() -> Void)?

    init(
        title: String,
        value: String,
        subtitle: String? = nil,
        tint: Color = Theme.accent,
        action: (() -> Void)? = nil
    ) {
        self.title = title
        self.value = value
        self.subtitle = subtitle
        self.tint = tint
        self.action = action
    }

    var body: some View {
        Button(action: { action?() }) {
            SurfaceCard {
                VStack(alignment: .leading, spacing: Theme.spacingXS) {
                    HStack(spacing: Theme.spacingSM) {
                        // A 3pt rule carries the metric's tint without relying on the
                        // number's colour alone to signal severity.
                        Capsule()
                            .fill(tint)
                            .frame(width: 3, height: 14)
                        Text(title)
                            .font(Theme.Typography.cardLabel)
                            .tracking(0.6)
                            .foregroundStyle(Theme.muted)
                            .textCase(.uppercase)
                            .lineLimit(1)
                    }

                    Text(value)
                        .font(Theme.Typography.metricValue)
                        .foregroundStyle(Theme.textPrimary)
                        .contentTransition(.numericText())
                        .lineLimit(1)
                        .minimumScaleFactor(0.6)

                    if let subtitle {
                        Text(subtitle)
                            .font(.caption2)
                            .foregroundStyle(Theme.muted)
                            .lineLimit(2)
                    }
                }
            }
        }
        .buttonStyle(.pressable)
        .disabled(action == nil)
        .accessibilityElement(children: .combine)
        .accessibilityLabel("\(title): \(value)\(subtitle.map { ", \($0)" } ?? "")")
    }
}

/// Status pill. `systemImage` defaults to an icon derived from the colour so the
/// badge never communicates by colour alone.
struct StatusBadge: View {
    let text: String
    let color: Color
    var systemImage: String?

    var body: some View {
        HStack(spacing: 4) {
            Image(systemName: systemImage ?? Theme.severityIcon(color))
                .font(.system(size: 9, weight: .bold))
            Text(text)
                .font(.caption2.weight(.semibold))
                .lineLimit(1)
        }
        .padding(.horizontal, 8)
        .padding(.vertical, 4)
        .background(color.opacity(0.18), in: Capsule())
        .overlay(Capsule().strokeBorder(color.opacity(0.45), lineWidth: 1))
        .foregroundStyle(Theme.onTint(for: color))
        .accessibilityLabel(text)
    }
}

struct HealthIndicator: View {
    let color: Color
    let label: String

    var body: some View {
        Circle()
            .fill(color)
            .frame(width: 10, height: 10)
            .overlay(Circle().stroke(color.opacity(0.35), lineWidth: 3).scaleEffect(1.6))
            .accessibilityLabel(label)
    }
}

// MARK: - Filters

struct FilterChipBar<Item: Hashable>: View {
    let items: [Item]
    @Binding var selection: Item
    let title: (Item) -> String

    var body: some View {
        ScrollView(.horizontal, showsIndicators: false) {
            HStack(spacing: Theme.spacingSM) {
                ForEach(items, id: \.self) { item in
                    let isSelected = selection == item
                    Button {
                        withAnimation(Theme.Motion.quick) { selection = item }
                    } label: {
                        Text(title(item))
                            .font(.footnote.weight(.semibold))
                            .lineLimit(1)
                            .padding(.horizontal, 14)
                            .padding(.vertical, 9)
                            .background(
                                isSelected ? Theme.accent.opacity(0.22) : Theme.surfaceElevated,
                                in: Capsule()
                            )
                            .overlay(
                                Capsule()
                                    .strokeBorder(isSelected ? Theme.accent : Theme.brandBorder, lineWidth: 1)
                            )
                            .foregroundStyle(isSelected ? Theme.accentLight : Theme.textSecondary)
                    }
                    .buttonStyle(.pressable)
                    .accessibilityAddTraits(isSelected ? .isSelected : [])
                }
            }
            .padding(.horizontal, Theme.spacingMD)
            .padding(.vertical, 2)
        }
    }
}

// MARK: - States

struct LoadingOverlay: View {
    let message: String

    var body: some View {
        VStack(spacing: Theme.spacingMD) {
            ProgressView()
                .controlSize(.large)
                .tint(Theme.accent)
            Text(message)
                .font(.subheadline)
                .foregroundStyle(Theme.textSecondary)
        }
        .frame(maxWidth: .infinity, maxHeight: .infinity)
        .accessibilityElement(children: .combine)
        .accessibilityLabel(message)
    }
}

struct ErrorBanner: View {
    let message: String
    let retry: (() -> Void)?

    var body: some View {
        VStack(spacing: Theme.spacingMD) {
            Image(systemName: "exclamationmark.triangle.fill")
                .font(.largeTitle)
                .foregroundStyle(Theme.danger)
                .symbolRenderingMode(.hierarchical)
            Text(message)
                .font(.subheadline)
                .multilineTextAlignment(.center)
                .foregroundStyle(Theme.textSecondary)
                .fixedSize(horizontal: false, vertical: true)
            if let retry {
                Button("Retry", action: retry)
                    .buttonStyle(.borderedProminent)
                    .tint(Theme.accent)
                    .controlSize(.large)
            }
        }
        .padding(Theme.spacingLG)
        .frame(maxWidth: .infinity)
    }
}

struct EmptyStateView: View {
    let title: String
    let systemImage: String
    var description: String?

    var body: some View {
        ContentUnavailableView {
            Label(title, systemImage: systemImage)
                .foregroundStyle(Theme.textPrimary)
        } description: {
            if let description {
                Text(description)
                    .foregroundStyle(Theme.muted)
            }
        }
        .frame(maxWidth: .infinity, maxHeight: .infinity)
    }
}

// MARK: - Controls

struct ThemedPrimaryButton: View {
    let title: String
    var isLoading = false
    let action: () -> Void

    var body: some View {
        Button(action: action) {
            HStack(spacing: Theme.spacingSM) {
                if isLoading {
                    ProgressView().tint(.white)
                }
                Text(title)
                    .fontWeight(.semibold)
            }
            .frame(maxWidth: .infinity)
            .frame(minHeight: Theme.minTouchTarget)
            .foregroundStyle(.white)
            .background(
                LinearGradient(
                    colors: [Theme.accentStrong, Theme.accentStrong.opacity(0.86)],
                    startPoint: .top,
                    endPoint: .bottom
                ),
                in: RoundedRectangle(cornerRadius: Theme.cornerRadiusSmall + 2, style: .continuous)
            )
            .overlay(
                RoundedRectangle(cornerRadius: Theme.cornerRadiusSmall + 2, style: .continuous)
                    .strokeBorder(.white.opacity(0.16), lineWidth: 1)
            )
            .elevation(Theme.Elevation.card)
        }
        .buttonStyle(.pressable)
        .disabled(isLoading)
        .opacity(isLoading ? 0.75 : 1)
    }
}

/// Outlined secondary action. Visually subordinate to `ThemedPrimaryButton` so a
/// screen still reads as having exactly one primary CTA.
struct ThemedSecondaryButton: View {
    let title: String
    var systemImage: String?
    let action: () -> Void

    var body: some View {
        Button(action: action) {
            HStack(spacing: Theme.spacingSM) {
                if let systemImage {
                    Image(systemName: systemImage)
                }
                Text(title).fontWeight(.semibold)
            }
            .frame(maxWidth: .infinity)
            .frame(minHeight: Theme.minTouchTarget)
            .foregroundStyle(Theme.accentLight)
            .background(Theme.surfaceElevated, in: RoundedRectangle(cornerRadius: Theme.cornerRadiusSmall + 2, style: .continuous))
            .overlay(
                RoundedRectangle(cornerRadius: Theme.cornerRadiusSmall + 2, style: .continuous)
                    .strokeBorder(Theme.brandBorderStrong, lineWidth: 1)
            )
        }
        .buttonStyle(.pressable)
    }
}

struct SearchBar: View {
    @Binding var text: String
    let placeholder: String

    var body: some View {
        HStack(spacing: Theme.spacingSM) {
            Image(systemName: "magnifyingglass")
                .foregroundStyle(Theme.muted)
            TextField("", text: $text, prompt: Text(placeholder).foregroundStyle(Theme.muted))
                .textInputAutocapitalization(.never)
                .autocorrectionDisabled()
                .foregroundStyle(Theme.textPrimary)
            if !text.isEmpty {
                Button {
                    text = ""
                } label: {
                    Image(systemName: "xmark.circle.fill")
                        .foregroundStyle(Theme.muted)
                }
                .accessibilityLabel("Clear search")
            }
        }
        .padding(.horizontal, 12)
        .frame(minHeight: Theme.minTouchTarget)
        .background(Theme.surfaceElevated, in: RoundedRectangle(cornerRadius: Theme.cornerRadiusSmall, style: .continuous))
        .overlay(
            RoundedRectangle(cornerRadius: Theme.cornerRadiusSmall, style: .continuous)
                .strokeBorder(Theme.brandBorder, lineWidth: 1)
        )
    }
}

struct ChatComposerBar: View {
    @Binding var text: String
    let placeholder: String
    var isSending = false
    let onSend: () -> Void

    var body: some View {
        HStack(alignment: .bottom, spacing: Theme.spacingSM) {
            TextField("", text: $text, prompt: Text(placeholder).foregroundStyle(Theme.muted), axis: .vertical)
                .lineLimit(1...4)
                .padding(.horizontal, 12)
                .padding(.vertical, 12)
                .background(Theme.surfaceElevated, in: RoundedRectangle(cornerRadius: Theme.cornerRadiusSmall, style: .continuous))
                .overlay(
                    RoundedRectangle(cornerRadius: Theme.cornerRadiusSmall, style: .continuous)
                        .strokeBorder(Theme.brandBorder, lineWidth: 1)
                )
                .foregroundStyle(Theme.textPrimary)

            Button(action: onSend) {
                Image(systemName: "arrow.up.circle.fill")
                    .font(.system(size: 32))
                    .symbolRenderingMode(.hierarchical)
                    .foregroundStyle(canSend ? Theme.accent : Theme.muted)
            }
            .frame(width: Theme.minTouchTarget, height: Theme.minTouchTarget)
            .disabled(!canSend || isSending)
            .accessibilityLabel("Send message")
        }
        .padding(Theme.spacingMD)
        .background(.ultraThinMaterial)
        .overlay(alignment: .top) {
            Rectangle().fill(Theme.brandBorder).frame(height: 1)
        }
    }

    private var canSend: Bool {
        !text.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty
    }
}
