import json
import os

SCHEDULE_PATH = os.path.join('config', 'schedule.json')

def save_schedule(resume_active):
    with open(SCHEDULE_PATH, 'w', encoding='utf-8') as f:
        json.dump(resume_active, f, ensure_ascii=False, indent=2)

def load_schedule():
    if not os.path.exists(SCHEDULE_PATH):
        return {}
    with open(SCHEDULE_PATH, 'r', encoding='utf-8') as f:
        return json.load(f)
