import json
import unittest

import verify


class OperatorFencingTests(unittest.TestCase):
    def test_operator_pods_are_checked_in_the_operator_namespace(self):
        for actual_pods, expected in (([], True), ([{'metadata': {'name': 'terminating-operator'}}], False)):
            c = object.__new__(verify.Cluster)
            calls = []

            def query(*args, **kwargs):
                calls.append((args, kwargs))
                if 'deployment' in args:
                    return json.dumps({'spec': {'replicas': 0}, 'status': {'replicas': 0}})
                return json.dumps({'items': actual_pods})

            c.k = query
            self.assertEqual(expected, c.operator_stopped())
            self.assertEqual(2, len(calls))
            for args, kwargs in calls:
                self.assertEqual(('-n', 'redis-operator-system'), args[:2])
                self.assertFalse(kwargs['namespace'])


if __name__ == '__main__':
    unittest.main()
