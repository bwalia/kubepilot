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
                    .stroke(Theme.brandBorder, lineWidth: 1)
            )
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
            Spacer()
            if let actionTitle, let action {
                Button(actionTitle, action: action)
                    .font(.caption.weight(.semibold))
                    .foregroundStyle(Theme.accentLight)
            }
        }
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

            VStack(alignment: .leading, spacing: 2) {
                Text(appState.authManager.activeAccount?.displayName ?? "Cluster")
                    .font(.subheadline.weight(.semibold))
                    .foregroundStyle(Theme.textPrimary)
                Text(appState.clusterManager.activeContextName)
                    .font(.caption)
                    .foregroundStyle(Theme.muted)
            }

            Spacer(minLength: 8)

            if !appState.clusterManager.selectedNamespace.isEmpty {
                Text(appState.clusterManager.selectedNamespace)
                    .font(.caption2.weight(.semibold))
                    .padding(.horizontal, 10)
                    .padding(.vertical, 5)
                    .background(Theme.accent.opacity(0.18), in: Capsule())
                    .foregroundStyle(Theme.accentLight)
            }
        }
        .padding(Theme.spacingMD)
        .background(.ultraThinMaterial, in: RoundedRectangle(cornerRadius: Theme.cornerRadiusSmall, style: .continuous))
        .overlay(
            RoundedRectangle(cornerRadius: Theme.cornerRadiusSmall, style: .continuous)
                .stroke(Theme.brandBorder, lineWidth: 1)
        )
    }
}

// MARK: - Filters & status

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
                        withAnimation(.snappy) { selection = item }
                    } label: {
                        Text(title(item))
                            .font(.caption.weight(.semibold))
                            .lineLimit(1)
                            .padding(.horizontal, 12)
                            .padding(.vertical, 8)
                            .background(
                                isSelected ? Theme.accent.opacity(0.22) : Theme.surfaceElevated,
                                in: Capsule()
                            )
                            .overlay(
                                Capsule()
                                    .stroke(isSelected ? Theme.accent : Theme.brandBorder, lineWidth: 1)
                            )
                            .foregroundStyle(isSelected ? Theme.accentLight : Theme.textSecondary)
                    }
                    .buttonStyle(.plain)
                    .accessibilityAddTraits(isSelected ? .isSelected : [])
                }
            }
            .padding(.horizontal, Theme.spacingMD)
        }
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

struct ThemedPrimaryButton: View {
    let title: String
    var isLoading = false
    let action: () -> Void

    var body: some View {
        Button(action: action) {
            HStack(spacing: Theme.spacingSM) {
                if isLoading {
                    ProgressView()
                        .tint(.white)
                }
                Text(title)
                    .fontWeight(.semibold)
            }
            .frame(maxWidth: .infinity)
            .padding(.vertical, 14)
        }
        .buttonStyle(.borderedProminent)
        .tint(Theme.accent)
        .controlSize(.large)
        .disabled(isLoading)
    }
}

struct ChatComposerBar: View {
    @Binding var text: String
    let placeholder: String
    var isSending = false
    let onSend: () -> Void

    var body: some View {
        HStack(alignment: .bottom, spacing: Theme.spacingSM) {
            TextField(placeholder, text: $text, axis: .vertical)
                .lineLimit(1...4)
                .padding(.horizontal, 12)
                .padding(.vertical, 10)
                .background(Theme.surfaceElevated, in: RoundedRectangle(cornerRadius: Theme.cornerRadiusSmall, style: .continuous))
                .overlay(
                    RoundedRectangle(cornerRadius: Theme.cornerRadiusSmall, style: .continuous)
                        .stroke(Theme.brandBorder, lineWidth: 1)
                )
                .foregroundStyle(Theme.textPrimary)

            Button(action: onSend) {
                Image(systemName: "arrow.up.circle.fill")
                    .font(.system(size: 32))
                    .symbolRenderingMode(.hierarchical)
                    .foregroundStyle(canSend ? Theme.accent : Theme.muted)
            }
            .disabled(!canSend || isSending)
            .accessibilityLabel("Send message")
        }
        .padding(Theme.spacingMD)
        .background(.ultraThinMaterial)
    }

    private var canSend: Bool {
        !text.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty
    }
}
