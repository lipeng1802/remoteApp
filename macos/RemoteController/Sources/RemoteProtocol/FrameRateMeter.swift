import Foundation

// Start at the first received image, excluding connection/TOFU wait time.
public struct FrameRateMeter {
    private var baseline: (count: Int, time: TimeInterval)?
    public init() {}
    public mutating func update(totalFrames: Int, now: TimeInterval) -> Double? {
        guard let previous = baseline else {
            baseline = (totalFrames, now)
            return nil
        }
        guard now >= previous.time, totalFrames >= previous.count else {
            baseline = (totalFrames, now)
            return nil
        }
        let elapsed = now - previous.time
        guard elapsed >= 1 else { return nil }
        baseline = (totalFrames, now)
        return Double(totalFrames - previous.count) / elapsed
    }
}