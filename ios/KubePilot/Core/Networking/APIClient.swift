import Foundation

enum APIError: LocalizedError, Sendable {
    case invalidURL
    case unauthorized
    case forbidden(String)
    case notFound
    case serverError(Int, String?)
    case decodingFailed(String)
    case network(Error)
    case offline

    var errorDescription: String? {
        switch self {
        case .invalidURL: "Invalid server URL."
        case .unauthorized: "Authentication required. Check your credentials."
        case .forbidden(let msg): msg
        case .notFound: "Resource not found."
        case .serverError(_, let msg): Self.friendlyServerMessage(msg)
        case .decodingFailed(let detail): "Failed to parse response: \(detail)"
        case .network(let err): err.localizedDescription
        case .offline: "You appear to be offline."
        }
    }

    private static func friendlyServerMessage(_ raw: String?) -> String {
        guard let raw, !raw.isEmpty else {
            return "KubePilot could not load cluster data."
        }
        if raw.contains("connection refused") || raw.contains("no route to host") || raw.contains("i/o timeout") {
            return "KubePilot is reachable, but the configured Kubernetes cluster is offline or unreachable. Ask your admin to fix the server kubeconfig."
        }
        if raw.contains("getting credentials") || raw.contains("Unauthorized") || raw.contains("401") {
            return "KubePilot is reachable, but cluster credentials expired. Re-authenticate the cluster on the server."
        }
        return raw
    }
}

enum HTTPMethod: String, Sendable {
    case get = "GET"
    case post = "POST"
    case put = "PUT"
    case delete = "DELETE"
}

struct APIRequest: Sendable {
    let path: String
    let method: HTTPMethod
    let query: [String: String]
    let body: Data?
    let acceptsText: Bool
    let timeout: TimeInterval

    init(
        path: String,
        method: HTTPMethod = .get,
        query: [String: String] = [:],
        body: Data? = nil,
        acceptsText: Bool = false,
        timeout: TimeInterval = 180
    ) {
        self.path = path
        self.method = method
        self.query = query
        self.body = body
        self.acceptsText = acceptsText
        self.timeout = timeout
    }
}

/// Thread-safe HTTP client for KubePilot backend APIs.
actor APIClient {
    static let shared = APIClient()

    private let session: URLSession
    private var baseURL: URL?
    private var authHeader: String?

    init(session: URLSession = .shared) {
        let config = URLSessionConfiguration.default
        config.httpShouldUsePipelining = true
        config.waitsForConnectivity = true
        config.timeoutIntervalForRequest = 180
        self.session = URLSession(configuration: config)
    }

    func configure(baseURL: URL, authHeader: String?) {
        self.baseURL = baseURL
        self.authHeader = authHeader
    }

    func healthCheck() async throws -> Bool {
        guard let baseURL else { throw APIError.invalidURL }
        var request = URLRequest(url: baseURL.appendingPathComponent("healthz"))
        request.httpMethod = HTTPMethod.get.rawValue
        let (_, response) = try await session.data(for: request)
        guard let http = response as? HTTPURLResponse else { return false }
        return (200..<300).contains(http.statusCode)
    }

    func send<T: Decodable>(_ apiRequest: APIRequest, as type: T.Type = T.self) async throws -> T {
        let data = try await sendRaw(apiRequest)
        do {
            let decoder = JSONDecoder.kubePilot
            return try decoder.decode(T.self, from: data)
        } catch {
            throw APIError.decodingFailed(error.localizedDescription)
        }
    }

    func sendText(_ apiRequest: APIRequest) async throws -> String {
        let data = try await sendRaw(apiRequest)
        guard let text = String(data: data, encoding: .utf8) else {
            throw APIError.decodingFailed("Response is not valid UTF-8 text.")
        }
        return text
    }

    func sendVoid(_ apiRequest: APIRequest) async throws {
        _ = try await sendRaw(apiRequest)
    }

    private func sendRaw(_ apiRequest: APIRequest) async throws -> Data {
        guard let baseURL else { throw APIError.invalidURL }
        guard var components = URLComponents(
            url: baseURL.appendingPathComponent("api/v1").appendingPathComponent(apiRequest.path),
            resolvingAgainstBaseURL: false
        ) else {
            throw APIError.invalidURL
        }
        if !apiRequest.query.isEmpty {
            components.queryItems = apiRequest.query
                .filter { !$0.value.isEmpty }
                .map { URLQueryItem(name: $0.key, value: $0.value) }
        }
        guard let url = components.url else { throw APIError.invalidURL }

        var request = URLRequest(url: url)
        request.httpMethod = apiRequest.method.rawValue
        request.timeoutInterval = apiRequest.timeout
        request.setValue("application/json", forHTTPHeaderField: "Accept")
        if let authHeader {
            request.setValue(authHeader, forHTTPHeaderField: "Authorization")
        }
        if let body = apiRequest.body {
            request.httpBody = body
            request.setValue("application/json", forHTTPHeaderField: "Content-Type")
        }

        let data: Data
        let response: URLResponse
        do {
            (data, response) = try await session.data(for: request)
        } catch {
            if (error as NSError).code == NSURLErrorNotConnectedToInternet {
                throw APIError.offline
            }
            throw APIError.network(error)
        }

        guard let http = response as? HTTPURLResponse else {
            throw APIError.network(URLError(.badServerResponse))
        }

        switch http.statusCode {
        case 200..<300:
            return data
        case 401:
            throw APIError.unauthorized
        case 403:
            let msg = (try? JSONDecoder().decode(APIErrorResponse.self, from: data))?.error
            throw APIError.forbidden(msg ?? "Forbidden")
        case 404:
            throw APIError.notFound
        default:
            let msg = (try? JSONDecoder().decode(APIErrorResponse.self, from: data))?.error
            throw APIError.serverError(http.statusCode, msg)
        }
    }
}

private struct APIErrorResponse: Decodable {
    let error: String?
}

extension JSONDecoder {
    static let kubePilot: JSONDecoder = {
        let decoder = JSONDecoder()
        decoder.dateDecodingStrategy = .custom { decoder in
            let container = try decoder.singleValueContainer()
            let value = try container.decode(String.self)
            let formatters = [ISO8601DateFormatter(), ISO8601DateFormatter.kubePilotFractional]
            for formatter in formatters {
                if let date = formatter.date(from: value) { return date }
            }
            throw DecodingError.dataCorruptedError(in: container, debugDescription: "Invalid date: \(value)")
        }
        return decoder
    }()
}

extension ISO8601DateFormatter {
    nonisolated(unsafe) static let kubePilotFractional: ISO8601DateFormatter = {
        let f = ISO8601DateFormatter()
        f.formatOptions = [.withInternetDateTime, .withFractionalSeconds]
        return f
    }()
}

extension JSONEncoder {
    static let kubePilot: JSONEncoder = {
        let encoder = JSONEncoder()
        encoder.dateEncodingStrategy = .iso8601
        return encoder
    }()
}
