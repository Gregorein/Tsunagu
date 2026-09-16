-- Extension chapter numbers cross a Kotlin Float -> Double widening on their
-- way into Go, which can turn clean values like 19.1 into noise like
-- 19.100000381469727. New writes are rounded in Go (chapternum.Round), but
-- existing rows need a one-time cleanup.
UPDATE chapters SET number = ROUND(number, 2) WHERE number IS NOT NULL;
