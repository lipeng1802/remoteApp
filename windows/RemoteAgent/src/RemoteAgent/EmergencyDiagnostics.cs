using System.IO;
using System.Threading.Channels;

namespace RemoteAgent;

// Opt-in, bounded and asynchronous: never perform file I/O in the keyboard hook.
internal static class EmergencyDiagnostics
{
    private static readonly Channel<string>? Queue = CreateQueue();
    private static Channel<string>? CreateQueue()
    {
        var directory = Environment.GetEnvironmentVariable("PRD_EMERGENCY_DIAGNOSTICS");
        if (string.IsNullOrWhiteSpace(directory)) return null;
        var queue = Channel.CreateBounded<string>(new BoundedChannelOptions(256)
        { FullMode = BoundedChannelFullMode.DropOldest, SingleReader = true });
        _ = Task.Run(async () =>
        {
            try
            {
                Directory.CreateDirectory(directory);
                using var writer = new StreamWriter(Path.Combine(directory, $"emergency-{Environment.ProcessId}.log")) { AutoFlush = true };
                await foreach (var line in queue.Reader.ReadAllAsync()) await writer.WriteLineAsync(line);
            }
            catch (IOException) { }
            catch (UnauthorizedAccessException) { }
        });
        return queue;
    }
    private static int remaining = 512;
    public static void Write(string message)
    {
        if (Queue is not null && Interlocked.Decrement(ref remaining) >= 0)
            Queue.Writer.TryWrite($"{DateTimeOffset.UtcNow:O} {message}");
    }
}
