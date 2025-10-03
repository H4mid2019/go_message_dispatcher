-- Initial schema for the message dispatcher system
-- This creates the messages table with optimized indexes for FIFO processing

-- Messages table stores all SMS messages with their delivery status
CREATE TABLE IF NOT EXISTS messages (
    id SERIAL PRIMARY KEY,
    phone_number VARCHAR(20) NOT NULL,
    content TEXT NOT NULL,
    sent BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMP DEFAULT NOW()
);

-- Index for efficient FIFO processing: finds unsent messages ordered by creation time
-- This is the most important index for the core functionality
CREATE INDEX IF NOT EXISTS idx_messages_sent_created ON messages(sent, created_at);

-- Index for phone number lookups (useful for debugging and analytics)
CREATE INDEX IF NOT EXISTS idx_messages_phone ON messages(phone_number);

-- Index for faster sent message queries
CREATE INDEX IF NOT EXISTS idx_messages_sent ON messages(sent) WHERE sent = TRUE;

-- Insert some sample messages for testing
INSERT INTO messages (phone_number, content) VALUES 
    ('+1234567890', 'Welcome to our service! Your account has been activated.'),
    ('+1234567891', 'Your verification code is: 123456'),
    ('+1234567892', 'Thank you for your purchase. Order #12345 has been confirmed.'),
    ('+1234567893', 'Reminder: Your appointment is scheduled for tomorrow at 2 PM.'),
    ('+1234567894', 'Your password has been successfully changed.'),
    ('+1234567895', 'Special offer: 20% off on all items. Use code SAVE20'),
    ('+1234567896', 'Your delivery is on the way and will arrive in 30 minutes.'),
    ('+1234567897', 'Account alert: Login detected from a new device.'),
    ('+1234567898', 'Congratulations! You have earned 100 reward points.'),
    ('+1234567899', 'System maintenance scheduled for tonight from 2-4 AM.')
ON CONFLICT DO NOTHING;

-- Add comments for documentation
COMMENT ON TABLE messages IS 'Stores SMS messages to be sent through the dispatcher system';
COMMENT ON COLUMN messages.sent IS 'Tracks whether the message has been successfully sent to prevent duplicates';
COMMENT ON INDEX idx_messages_sent_created IS 'Optimizes FIFO message processing queries';