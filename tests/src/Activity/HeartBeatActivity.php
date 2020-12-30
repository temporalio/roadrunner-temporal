<?php


namespace Temporal\Tests\Activity;

use Temporal\Client\Activity;
use Temporal\Client\Activity\ActivityInterface;
use Temporal\Client\Activity\ActivityMethod;

#[ActivityInterface(prefix: "HeartBeatActivity.")]
class HeartBeatActivity
{
    #[ActivityMethod]
    public function doSomething(int $value): string
    {
        Activity::heartbeat(['value' => $value]);
        sleep($value);
        return 'OK';
    }
}