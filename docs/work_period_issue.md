The obvious issue is the extremely large duration values — some cells show times like 0:43:02, 1:06:52, or even multi-hour durations. These are unrealistic for individual progress points that should take minutes, not hours.

The problem is that the duration is being calculated from the start key event to the end trigger event, but students don't work continuously between those two events. A student might start a progress point, close the game (or take a break, switch devices, come back the next day), and then finish it. The elapsed wall-clock time between start and end captures all that idle/offline time, making the durations meaningless as a measure of actual time spent working on the activity.

To get useful completion times, you'd need to account for gaps — either by using only active play session time between the start and end events, or by capping/filtering out durations that exceed a reasonable threshold for continuous play.

