INSERT INTO sagas (id, status, date_created) VALUES
	('45b5fbd3-755f-4379-8f07-a58d4a30fa2f', 'started', '2019-03-24 00:00:00')
	ON CONFLICT DO NOTHING;