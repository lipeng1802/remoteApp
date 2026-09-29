import XCTest
@testable import RemoteProtocol

final class FrameRateMeterTests: XCTestCase {
    func testExcludesConnectionWait() {
        var meter = FrameRateMeter()
        XCTAssertNil(meter.update(totalFrames: 1, now: 180))
        XCTAssertEqual(meter.update(totalFrames: 11, now: 181), 10)
    }
    func testReportsSustainedSlowFramesHonestly() {
        var meter = FrameRateMeter()
        XCTAssertNil(meter.update(totalFrames: 1, now: 20))
        XCTAssertEqual(meter.update(totalFrames: 2, now: 25), 0.2)
    }
    func testResetsOnCounterOrClockReset() {
        var meter = FrameRateMeter()
        XCTAssertNil(meter.update(totalFrames: 10, now: 10))
        XCTAssertNil(meter.update(totalFrames: 1, now: 11))
        XCTAssertEqual(meter.update(totalFrames: 6, now: 12), 5)
        XCTAssertNil(meter.update(totalFrames: 7, now: 1))
    }
}